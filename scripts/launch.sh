#!/bin/bash
#
# Lorag Launcher Script
# Usage: ./launch.sh [port]
#

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
PORT="${1:-8080}"
HOST="${LORAG_HOST:-localhost}"
DATA_DIR="${LORAG_DATA_DIR:-$PROJECT_ROOT/data}"

# Functions
print_banner() {
    echo -e "${BLUE}"
    echo "╔════════════════════════════════════════╗"
    echo "║              LORAG                     ║"
    echo "║     Local RAG + MCP Engine            ║"
    echo "╚════════════════════════════════════════╝"
    echo -e "${NC}"
}

check_dependencies() {
    echo -e "${YELLOW}Vérification des dépendances...${NC}"
    
    # Check if lorag binary exists
    if [ ! -f "$PROJECT_ROOT/bin/lorag" ]; then
        echo -e "${YELLOW}Binaire lorag non trouvé, compilation...${NC}"
        cd "$PROJECT_ROOT"
        make build-server
    fi
    
    # Check if Ollama is running (optional)
    if command -v ollama &> /dev/null; then
        if curl -s http://localhost:11434/api/tags > /dev/null 2>&1; then
            echo -e "${GREEN}✓ Ollama est en cours d'exécution${NC}"
        else
            echo -e "${YELLOW}⚠ Ollama est installé mais ne semble pas démarré${NC}"
            echo -e "${YELLOW}  Démarrez-le avec: ollama serve${NC}"
        fi
    else
        echo -e "${YELLOW}⚠ Ollama n'est pas installé${NC}"
        echo -e "${YELLOW}  Installez-le depuis: https://ollama.com${NC}"
    fi
}

setup_data_dir() {
    if [ ! -d "$DATA_DIR" ]; then
        echo -e "${YELLOW}Création du dossier de données: $DATA_DIR${NC}"
        mkdir -p "$DATA_DIR"
    fi
}

start_server() {
    echo -e "${GREEN}Démarrage de Lorag...${NC}"
    echo -e "${BLUE}  Host: $HOST${NC}"
    echo -e "${BLUE}  Port: $PORT${NC}"
    echo -e "${BLUE}  Data: $DATA_DIR${NC}"
    echo ""
    
    export LORAG_HOST="$HOST"
    export LORAG_PORT="$PORT"
    export LORAG_DATA_DIR="$DATA_DIR"
    
    # Create log directory
    mkdir -p "$DATA_DIR/logs"
    
    echo -e "${GREEN}Serveur démarré !${NC}"
    echo -e "${BLUE}  Web UI:   http://$HOST:$PORT${NC}"
    echo -e "${BLUE}  API:      http://$HOST:$PORT/api/v1/${NC}"
    echo -e "${BLUE}  MCP:      http://$HOST:$PORT/mcp${NC}"
    echo ""
    echo -e "${YELLOW}Appuyez sur Ctrl+C pour arrêter${NC}"
    echo ""
    
    # Open browser (macOS)
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sleep 2 && open "http://$HOST:$PORT" &
    fi
    
    # Start server
    "$PROJECT_ROOT/bin/lorag" server
}

# Main
print_banner
check_dependencies
setup_data_dir
start_server
