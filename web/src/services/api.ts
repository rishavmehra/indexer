import axios from 'axios';

const API_BASE_URL = import.meta.env.VITE_BACKEND_API_BASE_URL;

const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Store logout function reference
let logoutFunction: (() => void) | null = null;

// Set up functions for handling auth expiration
export const setupAuthExpirationHandling = (logout: () => void) => {
  logoutFunction = logout;
};

// Add interceptor to include auth token in requests
api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('token');
    if (token) {
      config.headers['Authorization'] = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// Add response interceptor to handle token expiration (401 Unauthorized)
api.interceptors.response.use(
  (response) => response,
  (error) => {
    // Check if error is due to unauthorized access (expired token)
    if (error.response && error.response.status === 401) {
      // Clear authentication data
      if (logoutFunction) {
        logoutFunction();
      }
      
      // Use window.location for navigation
      const path = window.location.pathname;
      if (path.startsWith('/dashboard')) {
        window.location.href = '/';
      }
    }
    
    return Promise.reject(error);
  }
);

// Auth service functions
export const AuthService = {
  signup: async (email: string, password: string) => {
    return api.post('/auth/signup', { email, password });
  },
  
  login: async (email: string, password: string) => {
    return api.post('/auth/login', { email, password });
  },
};

// User service functions
export const UserService = {
  getCurrentUser: async () => {
    return api.get('/users/me');
  },
};

// Database Credentials service functions
export const DBCredentialService = {
  getAll: async () => {
    return api.get('/users/db-credentials');
  },
  
  getById: async (id: string) => {
    return api.get(`/users/db-credentials/${id}`);
  },
  
  create: async (data: {
    host: string;
    port: number;
    name: string;
    user: string;
    password: string;
    sslMode?: string;
  }) => {
    return api.post('/users/db-credentials', data);
  },
  
  update: async (id: string, data: {
    host: string;
    port: number;
    name: string;
    user: string;
    password: string;
    sslMode?: string;
  }) => {
    return api.put(`/users/db-credentials/${id}`, data);
  },
  
  delete: async (id: string) => {
    return api.delete(`/users/db-credentials/${id}`);
  },

  testConnection: async (data: {
    host: string;
    port: number;
    name: string;
    user: string;
    password: string;
    sslMode?: string;
  }) => {
    return api.post('/users/db-credentials/test', data);
  }
};

// Indexer service functions
export const IndexerService = {
  getAll: async () => {
    return api.get('/indexers');
  },
  
  getById: async (id: string) => {
    return api.get(`/indexers/${id}`);
  },
  
  create: async (data: {
    dbCredentialId: string;
    indexerType: string;
    targetTable: string;
    params: {
      collection?: string;
      marketplaces?: string[];
      tokens?: string[];
      platforms?: string[];
    };
  }) => {
    return api.post('/indexers', data);
  },
  
  pause: async (id: string) => {
    return api.post(`/indexers/${id}/pause`);
  },
  
  resume: async (id: string) => {
    return api.post(`/indexers/${id}/resume`);
  },
  
  delete: async (id: string) => {
    return api.delete(`/indexers/${id}`);
  },
  
  getLogs: async (id: string, limit: number = 100, offset: number = 0) => {
    return api.get(`/indexers/${id}/logs?limit=${limit}&offset=${offset}`);
  },
  
  debugWebhook: async (webhookId: string) => {
    return api.get(`/indexers/debug/webhook/${webhookId}`);
  },
  
  testProcess: async (data: {
    webhookId: string;
    data: any;
  }) => {
    return api.post('/indexers/test-process', data);
  }
};

export default api;
