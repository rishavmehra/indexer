import { useEffect } from "react";
import { BrowserRouter as Router, Routes, Route, Navigate } from "react-router-dom";
import { Navbar } from "./components/Navbar";
import { Footer } from "./components/Footer";
import { ScrollToTop } from "./components/ScrollToTop";
import { useAuth } from "./context/AuthContext";
import { setupAuthExpirationHandling } from "./services/api";
import HomePage from "./pages/HomePage";
import LoginPage from "./pages/LoginPage";
import SignupPage from "./pages/SignupPage";
import DashboardPage from "./pages/DashboardPage";
import DashboardLayout from "./components/dashboard/DashboardLayout";
import DatabaseCredentialsPage from "./pages/DatabaseCredentialsPage";
import AddCredentialPage from "./pages/AddCredentialPage";
import EditCredentialPage from "./pages/EditCredentialPage";
import SettingsPage from "./pages/SettingsPage";
import IndexersPage from "./pages/IndexersPage";
import CreateIndexerPage from "./pages/CreateIndexerPage";
import IndexerLogsPage from "./pages/IndexerLogsPage";
import ProtectedRoute from "./components/ProtectedRoute";
import "./App.css";

function AppContent() {
  const { isAuthenticated, logout } = useAuth();
  
  // Set up auth expiration handling
  useEffect(() => {
    setupAuthExpirationHandling(logout);
  }, [logout]);
  
  return (
    <>
      {/* Only show Navbar on non-dashboard pages */}
      <Routes>
        <Route path="/dashboard/*" element={null} />
        <Route path="*" element={<Navbar />} />
      </Routes>
      
      <Routes>
        <Route path="/" element={<HomePage />} />
        <Route path="/login" element={isAuthenticated ? <Navigate to="/dashboard" /> : <LoginPage />} />
        <Route path="/signup" element={isAuthenticated ? <Navigate to="/dashboard" /> : <SignupPage />} />
        
        {/* Dashboard Routes */}
        <Route path="/dashboard" element={
          <ProtectedRoute>
            <DashboardLayout />
          </ProtectedRoute>
        }>
          <Route index element={<DashboardPage />} />
          
          {/* Indexer Routes */}
          <Route path="indexers" element={<IndexersPage />} />
          <Route path="indexers/create" element={<CreateIndexerPage />} />
          <Route path="indexers/:id/logs" element={<IndexerLogsPage />} />
          
          {/* DB Credential Routes */}
          <Route path="db-credentials" element={<DatabaseCredentialsPage />} />
          <Route path="db-credentials/add" element={<AddCredentialPage />} />
          <Route path="db-credentials/edit/:id" element={<EditCredentialPage />} />
          
          {/* Settings */}
          <Route path="settings" element={<SettingsPage />} />
        </Route>
      </Routes>
      
      {/* Only show Footer on non-dashboard pages */}
      <Routes>
        <Route path="/dashboard/*" element={null} />
        <Route path="*" element={<Footer />} />
      </Routes>
      
      <ScrollToTop />
    </>
  );
}

function App() {
  return (
    <Router>
      <AppContent />
    </Router>
  );
}

export default App;
