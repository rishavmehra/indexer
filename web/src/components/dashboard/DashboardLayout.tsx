import React from 'react';
import { Outlet } from 'react-router-dom';
import DashboardSidebar from './DashboardSidebar';
import { ToastProvider } from '@/components/ui/toast';
import { ScrollToTop } from '@/components/ScrollToTop';
import { motion } from 'framer-motion';
import { Bell, Search } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

const DashboardLayout: React.FC = () => {
  return (
    <div className="flex h-screen bg-background overflow-hidden">
      <DashboardSidebar />
      <main className="flex-1 flex flex-col overflow-hidden">
        <ToastProvider>
          {/* Top navigation bar */}
          <header className="h-16 border-b bg-card/50 backdrop-blur-sm flex items-center px-6 sticky top-0 z-20">
            <div className="flex items-center w-full justify-between">
              <div className="relative w-64">
                <Search className="absolute left-3 top-3 h-4 w-4 text-muted-foreground" />
                <Input 
                  placeholder="Search..." 
                  className="pl-10 w-full bg-background" 
                />
              </div>
              <div className="flex items-center gap-4">
                <Button variant="ghost" size="icon" className="relative">
                  <Bell className="h-5 w-5" />
                  <span className="absolute -top-1 -right-1 w-4 h-4 bg-primary rounded-full text-white text-[10px] flex items-center justify-center">
                    2
                  </span>
                </Button>
              </div>
            </div>
          </header>

          {/* Main content area with motion animation */}
          <motion.div 
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ duration: 0.3 }}
            className="flex-1 overflow-y-auto"
            key={window.location.pathname}
          >
            <ScrollToTop />
            <Outlet />
          </motion.div>
        </ToastProvider>
      </main>
    </div>
  );
};

export default DashboardLayout;

