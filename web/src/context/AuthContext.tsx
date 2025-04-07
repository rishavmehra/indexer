import React, { createContext, useContext, useState, useEffect } from 'react';

interface AuthContextType {
  isAuthenticated: boolean;
  user: any | null;
  token: string | null;
  login: (token: string, user: any) => void;
  logout: () => void;
  checkTokenExpiration: () => boolean;
}

const AuthContext = createContext<AuthContextType>({
  isAuthenticated: false,
  user: null,
  token: null,
  login: () => {},
  logout: () => {},
  checkTokenExpiration: () => false,
});

// Helper function to decode JWT token without verification
const parseJwt = (token: string) => {
  try {
    return JSON.parse(atob(token.split('.')[1]));
  } catch (e) {
    return null;
  }
};

// Helper function to check if token is expired
const isTokenExpired = (token: string) => {
  const decodedToken = parseJwt(token);
  if (!decodedToken) return true;
  
  // Get current time in seconds
  const currentTime = Math.floor(Date.now() / 1000);
  
  // Check if token is expired (exp is in seconds)
  return decodedToken.exp < currentTime;
};

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [token, setToken] = useState<string | null>(localStorage.getItem('token'));
  const [user, setUser] = useState<any | null>(
    localStorage.getItem('user') ? JSON.parse(localStorage.getItem('user') || '{}') : null
  );
  const [isAuthenticated, setIsAuthenticated] = useState<boolean>(false);

  // Function to check token expiration
  const checkTokenExpiration = () => {
    if (!token) return false;
    
    const expired = isTokenExpired(token);
    if (expired) {
      // If token is expired, logout the user
      logout();
      return false;
    }
    
    return true;
  };

  // Effect to set authentication state based on token
  useEffect(() => {
    if (token) {
      // Check if token is valid and not expired
      const isValid = !isTokenExpired(token);
      
      if (isValid) {
        setIsAuthenticated(true);
      } else {
        // If token is expired, clean up
        localStorage.removeItem('token');
        localStorage.removeItem('user');
        setToken(null);
        setUser(null);
        setIsAuthenticated(false);
        
        // Redirect to home page if we're on a protected route
        const path = window.location.pathname;
        if (path.startsWith('/dashboard')) {
          window.location.href = '/';
        }
      }
    } else {
      setIsAuthenticated(false);
    }
  }, [token]);

  // Set up periodic token validation (every minute)
  useEffect(() => {
    const validateToken = () => {
      if (token && isTokenExpired(token)) {
        logout();
        
        // Use window.location for navigation since we can't use useNavigate here
        const path = window.location.pathname;
        if (path.startsWith('/dashboard')) {
          window.location.href = '/';
        }
      }
    };
    
    const interval = setInterval(validateToken, 60000); // Check every minute
    
    return () => clearInterval(interval);
  }, [token]);

  const login = (newToken: string, userData: any) => {
    localStorage.setItem('token', newToken);
    localStorage.setItem('user', JSON.stringify(userData));
    setToken(newToken);
    setUser(userData);
    setIsAuthenticated(true);
  };

  const logout = () => {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    setToken(null);
    setUser(null);
    setIsAuthenticated(false);
  };

  return (
    <AuthContext.Provider value={{ 
      isAuthenticated, 
      user, 
      token, 
      login, 
      logout,
      checkTokenExpiration
    }}>
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => useContext(AuthContext);
