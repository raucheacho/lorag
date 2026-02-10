import { useState } from 'react';
import { 
  LayoutDashboard, 
  Search, 
  FileText, 
  FolderOpen, 
  Settings,
  Database,
  Activity,
  Sparkles,
  FileCode,
  HelpCircle
} from 'lucide-react';

interface NavItem {
  id: string;
  label: string;
  icon: React.ReactNode;
  category?: string;
}

const navItems: NavItem[] = [
  { id: 'dashboard', label: 'Tableau de bord', icon: <LayoutDashboard size={18} /> },
  { id: 'search', label: 'Recherche', icon: <Search size={18} /> },
  { id: 'documents', label: 'Documents', icon: <FileText size={18} /> },
  { id: 'sources', label: 'Sources', icon: <FolderOpen size={18} /> },
  { id: 'templates', label: 'Templates', icon: <FileCode size={18} /> },
  { id: 'settings', label: 'Paramètres', icon: <Settings size={18} /> },
];

interface LayoutProps {
  children: React.ReactNode;
  activePage: string;
  onPageChange: (page: string) => void;
  showWizard: () => void;
}

export function Layout({ children, activePage, onPageChange, showWizard }: LayoutProps) {
  const [collapsed] = useState(false);

  return (
    <div className="flex h-screen bg-bg">
      {/* Sidebar */}
      <aside 
        className={`${collapsed ? 'w-16' : 'w-64'} 
          bg-bg-secondary border-r border-border flex flex-col
          transition-all duration-300 ease-in-out`}
      >
        {/* Logo */}
        <div className="h-14 flex items-center px-4 border-b border-border">
          <Database className="text-accent w-6 h-6 flex-shrink-0" />
          {!collapsed && (
            <span className="ml-3 font-semibold text-lg tracking-tight">
              Lorag
            </span>
          )}
        </div>

        {/* Navigation */}
        <nav className="flex-1 py-4 px-2 space-y-1">
          {navItems.map((item) => (
            <button
              key={item.id}
              onClick={() => onPageChange(item.id)}
              className={`w-full flex items-center px-3 py-2 rounded-md text-sm
                transition-colors duration-200
                ${activePage === item.id 
                  ? 'bg-accent-muted text-accent border border-accent/20' 
                  : 'text-text-secondary hover:text-text hover:bg-bg-tertiary'
                }
                ${collapsed ? 'justify-center' : 'justify-start'}`}
            >
              <span className="flex-shrink-0">{item.icon}</span>
              {!collapsed && <span className="ml-3">{item.label}</span>}
            </button>
          ))}
        </nav>

        {/* Help section */}
        <div className="px-2 pb-2">
          <button
            onClick={showWizard}
            className={`w-full flex items-center px-3 py-2 rounded-md text-sm
              text-text-secondary hover:text-text hover:bg-bg-tertiary
              transition-colors duration-200
              ${collapsed ? 'justify-center' : 'justify-start'}`}
          >
            <HelpCircle size={18} />
            {!collapsed && <span className="ml-3">Relancer le guide</span>}
          </button>
        </div>

        {/* Status footer */}
        <div className="p-4 border-t border-border">
          <div className="flex items-center gap-2">
            <div className="w-2 h-2 rounded-full bg-status-success animate-pulse" />
            {!collapsed && (
              <span className="text-xs text-text-tertiary font-mono">
                En ligne
              </span>
            )}
          </div>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 flex flex-col overflow-hidden">
        {/* Header */}
        <header className="h-14 border-b border-border bg-bg-secondary/50 
          backdrop-blur flex items-center justify-between px-6">
          <h1 className="text-lg font-medium capitalize">
            {navItems.find(i => i.id === activePage)?.label || activePage}
          </h1>
          <div className="flex items-center gap-4">
            <button
              onClick={showWizard}
              className="flex items-center gap-2 px-3 py-1.5 text-sm text-accent 
                hover:bg-accent/10 rounded-lg transition-colors"
            >
              <Sparkles size={14} />
              <span>Assistant</span>
            </button>
            <div className="flex items-center gap-2 text-xs text-text-tertiary">
              <Activity size={14} />
              <span>v0.1.0</span>
            </div>
          </div>
        </header>

        {/* Page content */}
        <div className="flex-1 overflow-auto p-6">
          {children}
        </div>
      </main>
    </div>
  );
}
