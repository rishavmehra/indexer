import React from 'react';
import { ThemeProvider } from '@/components/theme-provider';
import { AuthProvider } from '@/context/AuthContext';
import { ToastProvider } from '@/components/ui/toast';
import { AnimatePresence } from 'framer-motion';

// Import global styling
import '@/global-animations.css';

interface AppProviderProps {
  children: React.ReactNode;
}

export const AppProvider: React.FC<AppProviderProps> = ({ children }) => {
  return (
    <ThemeProvider>
      <AuthProvider>
        <ToastProvider>
          <AnimatePresence mode="wait">
            {children}
          </AnimatePresence>
        </ToastProvider>
      </AuthProvider>
    </ThemeProvider>
  );
};