#!/usr/bin/env python3
"""
Example MCP client for Lorag.

This example demonstrates how to use the MCP protocol to interact
with the Lorag RAG engine from Python.
"""

import requests
import json
import sys

MCP_URL = "http://localhost:8080/mcp"


def mcp_request(method: str, params: dict = None, request_id: int = 1) -> dict:
    """Send an MCP request."""
    payload = {
        "jsonrpc": "2.0",
        "method": method,
        "id": request_id
    }
    if params:
        payload["params"] = params
    
    response = requests.post(MCP_URL, json=payload)
    response.raise_for_status()
    return response.json()


def initialize():
    """Initialize the MCP connection."""
    result = mcp_request("initialize")
    print(f"Connected to: {result['result']['name']} v{result['result']['version']}")
    print(f"Capabilities: {', '.join(result['result']['capabilities'])}")
    return result


def list_tools():
    """List available tools."""
    result = mcp_request("tools/list")
    tools = result['result']['tools']
    print(f"\nAvailable tools ({len(tools)}):")
    for tool in tools:
        print(f"  - {tool['name']}: {tool['description']}")
    return tools


def search(query: str, top_k: int = 5):
    """Search indexed documents."""
    result = mcp_request("tools/call", {
        "name": "search",
        "arguments": {"query": query, "top_k": top_k}
    })
    
    content = result['result']['content'][0]['text']
    print(f"\n{content}")
    return content


def index_file(path: str, recursive: bool = False):
    """Index a file or directory."""
    result = mcp_request("tools/call", {
        "name": "index_file",
        "arguments": {"path": path, "recursive": recursive}
    })
    
    content = result['result']['content'][0]['text']
    print(f"\n{content}")
    return content


def get_stats():
    """Get indexing statistics."""
    result = mcp_request("tools/call", {
        "name": "get_stats",
        "arguments": {}
    })
    
    content = result['result']['content'][0]['text']
    print(f"\n{content}")
    return content


def interactive_mode():
    """Run interactive MCP client."""
    print("Lorag MCP Client")
    print("================\n")
    
    # Initialize
    try:
        initialize()
        list_tools()
    except requests.exceptions.ConnectionError:
        print("Error: Could not connect to Lorag server at", MCP_URL)
        print("Make sure the server is running: lorag server")
        sys.exit(1)
    
    # Interactive loop
    while True:
        print("\nCommands:")
        print("  search <query>  - Search documents")
        print("  index <path>    - Index file or directory")
        print("  stats           - Show statistics")
        print("  tools           - List available tools")
        print("  quit            - Exit")
        
        try:
            command = input("\n> ").strip()
        except (EOFError, KeyboardInterrupt):
            print("\nGoodbye!")
            break
        
        if not command:
            continue
        
        parts = command.split(maxsplit=1)
        cmd = parts[0].lower()
        arg = parts[1] if len(parts) > 1 else ""
        
        try:
            if cmd == "search":
                if not arg:
                    arg = input("Enter search query: ")
                search(arg)
            
            elif cmd == "index":
                if not arg:
                    arg = input("Enter path to index: ")
                recursive = input("Recursive? (y/N): ").lower().startswith('y')
                index_file(arg, recursive)
            
            elif cmd == "stats":
                get_stats()
            
            elif cmd == "tools":
                list_tools()
            
            elif cmd in ("quit", "exit", "q"):
                print("Goodbye!")
                break
            
            else:
                print(f"Unknown command: {cmd}")
        
        except Exception as e:
            print(f"Error: {e}")


def main():
    """Main entry point."""
    if len(sys.argv) > 1:
        # Command mode
        command = sys.argv[1]
        
        if command == "search" and len(sys.argv) > 2:
            initialize()
            search(" ".join(sys.argv[2:]))
        
        elif command == "index" and len(sys.argv) > 2:
            initialize()
            recursive = "--recursive" in sys.argv or "-r" in sys.argv
            index_file(sys.argv[2], recursive)
        
        elif command == "stats":
            initialize()
            get_stats()
        
        else:
            print("Usage:")
            print("  python mcp-client.py search <query>")
            print("  python mcp-client.py index <path> [--recursive]")
            print("  python mcp-client.py stats")
            print("  python mcp-client.py")
    else:
        # Interactive mode
        interactive_mode()


if __name__ == "__main__":
    main()
