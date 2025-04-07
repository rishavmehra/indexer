import React, { useEffect } from 'react';
import { Navigate } from 'react-router-dom';
import { useAuth } from '@/context/AuthContext';
import { useToast } from '@/components/ui/toast';

interface ProtectedRouteProps {
  children: React.ReactNode;
}

const ProtectedRoute: React.FC<ProtectedRouteProps> = ({ children }) => {
  const { isAuthenticated, checkTokenExpiration } = useAuth();
  const { addToast } = useToast();
  
  useEffect(() => {
    // Check token validity on component mount
    if (!checkTokenExpiration()) {
      // Token is invalid or expired
      addToast({
        title: 'Session Expired',
        description: 'Your login session has expired. Please sign in again.',
        type: 'error'
      });
      // Use window.location instead of useNavigate
      window.location.href = '/';
    }
  }, [checkTokenExpiration, addToast]);
  
  if (!isAuthenticated) {
    // If not authenticated, redirect to login page
    return <Navigate to="/login" replace />;
  }
  
  return <>{children}</>;
};

export default ProtectedRoute;
