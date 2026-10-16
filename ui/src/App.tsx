import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import { Shell } from './components/Shell';
import { ToastProvider } from './components/toast';
import { OverviewPage } from './pages/Overview';
import { ExplorerPage } from './pages/Explorer';
import { PlaygroundPage } from './pages/Playground';
import { LogsPage } from './pages/Logs';
import { IngestionPage } from './pages/Ingestion';
import { ConnectPage } from './pages/Connect';
import { SettingsPage } from './pages/Settings';

export default function App() {
  return (
    <BrowserRouter>
      <ToastProvider>
        <Shell>
          <Routes>
            <Route path="/" element={<OverviewPage />} />
            <Route path="/explorer" element={<ExplorerPage />} />
            <Route path="/explorer/:id" element={<ExplorerPage />} />
            <Route path="/playground" element={<PlaygroundPage />} />
            <Route path="/logs" element={<LogsPage />} />
            <Route path="/ingestion" element={<IngestionPage />} />
            <Route path="/connect" element={<ConnectPage />} />
            <Route path="/settings" element={<SettingsPage />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </Shell>
      </ToastProvider>
    </BrowserRouter>
  );
}
