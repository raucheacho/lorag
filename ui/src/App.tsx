import { useState, useEffect } from 'react';
import { BrowserRouter, Routes, Route, useNavigate, useLocation } from 'react-router-dom';
import { Layout } from './components/Layout';
import { Wizard } from './components/Wizard';
import { DashboardPage } from './pages/DashboardPage';
import { DocumentsPage } from './pages/DocumentsPage';
import { DocumentDetailPage } from './pages/DocumentDetailPage';
import { SearchPage } from './pages/SearchPage';
import { SourcesPage } from './pages/SourcesPage';
import { SettingsPage } from './pages/SettingsPage';
import { TemplatesPage } from './pages/TemplatesPage';
import { Toaster } from './components/Toaster';

// Wrapper component to handle navigation and layout
function AppContent() {
  const [showWizard, setShowWizard] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();

  // Determine active page from URL
  const getActivePage = () => {
    const path = location.pathname;
    if (path.startsWith('/documents/')) return 'documents';
    if (path === '/documents') return 'documents';
    if (path === '/search') return 'search';
    if (path === '/sources') return 'sources';
    if (path === '/templates') return 'templates';
    if (path === '/settings') return 'settings';
    return 'dashboard';
  };

  const [activePage, setActivePage] = useState(getActivePage());

  useEffect(() => {
    setActivePage(getActivePage());
  }, [location]);

  useEffect(() => {
    // Check if first launch
    const hasCompletedWizard = localStorage.getItem('lorag-wizard-completed');
    if (!hasCompletedWizard) {
      setShowWizard(true);
    }
  }, []);

  const handleWizardComplete = () => {
    localStorage.setItem('lorag-wizard-completed', 'true');
    setShowWizard(false);
  };

  const handlePageChange = (page: string) => {
    navigate(`/${page}`);
  };

  return (
    <>
      {showWizard && <Wizard onComplete={handleWizardComplete} />}
      <Layout 
        activePage={activePage} 
        onPageChange={handlePageChange}
        showWizard={() => setShowWizard(true)}
      >
        <Routes>
          <Route path="/" element={<DashboardPage />} />
          <Route path="/dashboard" element={<DashboardPage />} />
          <Route path="/documents" element={<DocumentsPage />} />
          <Route path="/documents/:id" element={<DocumentDetailPage />} />
          <Route path="/search" element={<SearchPage />} />
          <Route path="/sources" element={<SourcesPage />} />
          <Route path="/templates" element={<TemplatesPage />} />
          <Route path="/settings" element={<SettingsPage />} />
        </Routes>
      </Layout>
      <Toaster />
    </>
  );
}

function App() {
  return (
    <BrowserRouter>
      <AppContent />
    </BrowserRouter>
  );
}

export default App;
