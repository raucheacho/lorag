import { useState } from 'react';
import { CheckCircle2, PlugZap, XCircle } from 'lucide-react';
import { mcp, McpTool } from '../lib/api';
import { useResource } from '../lib/hooks';
import { ms } from '../lib/format';
import { Badge, Button, Code, CopyButton, cx, PageHeader, Section } from '../components/ui';

interface Snippet {
  id: string;
  label: string;
  where: string;
  code: (url: string) => string;
}

const snippets: Snippet[] = [
  {
    id: 'http',
    label: 'HTTP clients',
    where: 'Any MCP client that accepts a remote (Streamable HTTP) server',
    code: (url) => JSON.stringify({ mcpServers: { lorag: { type: 'http', url } } }, null, 2),
  },
  {
    id: 'cursor',
    label: 'Cursor',
    where: '~/.cursor/mcp.json, or .cursor/mcp.json in a project',
    code: (url) => JSON.stringify({ mcpServers: { lorag: { url } } }, null, 2),
  },
  {
    id: 'vscode',
    label: 'VS Code',
    where: '.vscode/mcp.json in the workspace',
    code: (url) => JSON.stringify({ servers: { lorag: { type: 'http', url } } }, null, 2),
  },
  {
    id: 'stdio',
    label: 'stdio only',
    where: 'Clients that can only start a local process: bridge through mcp-remote (Node.js)',
    code: (url) => JSON.stringify({ mcpServers: { lorag: { command: 'npx', args: ['-y', 'mcp-remote', url] } } }, null, 2),
  },
  {
    id: 'curl',
    label: 'curl',
    where: 'Raw JSON-RPC, to script or debug',
    code: (url) =>
      `curl -s ${url} -H 'Content-Type: application/json' -d '{\n  "jsonrpc": "2.0", "id": 1, "method": "tools/call",\n  "params": {"name": "search", "arguments": {"query": "deploy rollback", "top_k": 5}}\n}'`,
  },
];

interface TestResult {
  ok: boolean;
  ms: number;
  server?: string;
  protocol?: string;
  tools?: number;
  error?: string;
}

export function ConnectPage() {
  const url = `${window.location.origin}/mcp`;
  const [tab, setTab] = useState(snippets[0].id);
  const [test, setTest] = useState<TestResult>();
  const [testing, setTesting] = useState(false);
  const tools = useResource(() => mcp<{ tools: McpTool[] }>('tools/list'), []);
  const snippet = snippets.find((s) => s.id === tab)!;

  const runTest = async () => {
    setTesting(true);
    const start = performance.now();
    try {
      const init = await mcp<{ protocolVersion: string; serverInfo: { name: string; version: string } }>('initialize', {
        protocolVersion: '2025-06-18',
        capabilities: {},
        clientInfo: { name: 'lorag-studio', version: '1' },
      });
      const list = await mcp<{ tools: McpTool[] }>('tools/list');
      setTest({
        ok: true,
        ms: performance.now() - start,
        server: `${init.serverInfo.name} ${init.serverInfo.version}`,
        protocol: init.protocolVersion,
        tools: list.tools.length,
      });
    } catch (e) {
      setTest({ ok: false, ms: performance.now() - start, error: e instanceof Error ? e.message : String(e) });
    } finally {
      setTesting(false);
    }
  };

  return (
    <div className="flex h-full flex-col overflow-y-auto">
      <PageHeader title="Connect" description="Give your agents access to this index over MCP" />
      <div className="mx-auto w-full max-w-4xl space-y-4 p-5">
        <Section title="MCP endpoint">
          <div className="flex flex-wrap items-center gap-3 px-4 py-3">
            <code className="flex-1 truncate rounded-md border border-line bg-raised px-3 py-1.5 font-mono text-[13px]">{url}</code>
            <CopyButton text={url} label="Copy" />
            <Button icon={<PlugZap size={14} />} disabled={testing} onClick={runTest}>
              {testing ? 'Testing…' : 'Test'}
            </Button>
          </div>
          {test && (
            <div className={cx('flex items-center gap-2 border-t px-4 py-2 text-xs', test.ok ? 'border-ok/30 bg-ok/5 text-ok' : 'border-bad/30 bg-bad/5 text-bad')}>
              {test.ok ? <CheckCircle2 size={13} /> : <XCircle size={13} />}
              {test.ok ? `${test.server} answered in ${ms(test.ms)} · protocol ${test.protocol} · ${test.tools} tools` : test.error}
            </div>
          )}
          <div className="border-t border-line px-4 py-2 text-xs text-faint">
            Streamable HTTP, JSON responses. Requests from web pages other than localhost are refused. Each client is named in the agent logs.
          </div>
        </Section>

        <Section title="Client configuration">
          <div className="flex gap-1 overflow-x-auto border-b border-line px-2">
            {snippets.map((s) => (
              <button
                key={s.id}
                onClick={() => setTab(s.id)}
                className={cx(
                  '-mb-px h-9 whitespace-nowrap border-b-2 px-3 text-xs font-medium',
                  tab === s.id ? 'border-accent text-fg' : 'border-transparent text-muted hover:text-fg',
                )}
              >
                {s.label}
              </button>
            ))}
          </div>
          <div className="space-y-2 p-4">
            <div className="text-xs text-faint">{snippet.where}</div>
            <Code copy>{snippet.code(url)}</Code>
          </div>
        </Section>

        <Section title={`Tools exposed${tools.data ? ` (${tools.data.tools.length})` : ''}`}>
          {tools.error && <div className="px-4 py-3 text-xs text-bad">{tools.error}</div>}
          <div className="divide-y divide-line">
            {tools.data?.tools.map((t) => (
              <div key={t.name} className="px-4 py-3">
                <div className="flex items-center gap-2">
                  <code className="font-mono text-[13px] font-semibold">{t.name}</code>
                  {t.annotations?.readOnlyHint ? <Badge tone="ok">read-only</Badge> : <Badge tone="warn">writes</Badge>}
                </div>
                <p className="mt-1 text-xs text-muted">{t.description}</p>
                {t.inputSchema.properties && Object.keys(t.inputSchema.properties).length > 0 && (
                  <table className="mt-2 w-full table-fixed text-xs">
                    <tbody>
                      {Object.entries(t.inputSchema.properties).map(([name, p]) => (
                        <tr key={name}>
                          <td className="w-36 py-1 pr-3 align-top font-mono">
                            {name}
                            {t.inputSchema.required?.includes(name) && <span className="text-bad">*</span>}
                          </td>
                          <td className="w-48 py-1 pr-3 align-top font-mono text-faint">{p.enum ? p.enum.join(' | ') : p.type}</td>
                          <td className="py-1 align-top text-muted">
                            {p.description}
                            {p.default !== undefined && <span className="ml-1 font-mono text-faint">(default {String(p.default)})</span>}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>
            ))}
          </div>
        </Section>
      </div>
    </div>
  );
}
