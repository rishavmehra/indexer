import React, { useState, useEffect } from 'react';
import { NavLink, useNavigate, Link } from 'react-router-dom';
import { Button } from '@/components/ui/button';
import { useAuth } from '@/context/AuthContext';
import {
    Home,
    BarChart3,
    Database,
    Settings,
    LogOut,
    ChevronLeft,
    ChevronRight,
    PlusCircle,
    User,
    AlertCircle
} from 'lucide-react';
import { LogoIcon } from '@/components/Icons';
import { ModeToggle } from '@/components/mode-toggle';
import { motion, AnimatePresence } from 'framer-motion';

const DashboardSidebar: React.FC = () => {
    const { logout, user } = useAuth();
    const navigate = useNavigate();
    const [collapsed, setCollapsed] = useState(() => {
        const saved = localStorage.getItem('sidebarCollapsed');
        return saved ? JSON.parse(saved) : false;
    });

    useEffect(() => {
        localStorage.setItem('sidebarCollapsed', JSON.stringify(collapsed));
    }, [collapsed]);

    const handleLogout = () => {
        logout();
        navigate('/');
    };

    const sidebarVariants = {
        expanded: { width: '240px' },
        collapsed: { width: '70px' }
    };

    const navItems = [
        {
            path: '/dashboard',
            icon: <Home className="h-5 w-5" />,
            label: 'Dashboard',
            exact: true
        },
        {
            path: '/dashboard/indexers',
            icon: <BarChart3 className="h-5 w-5" />,
            label: 'Blockchain Indexers'
        },
        {
            path: '/dashboard/db-credentials',
            icon: <Database className="h-5 w-5" />,
            label: 'Database Credentials'
        },
        {
            path: '/dashboard/settings',
            icon: <Settings className="h-5 w-5" />,
            label: 'Settings'
        },
    ];

    const quickActions = [
        {
            path: '/dashboard/indexers/create',
            icon: <PlusCircle className="h-5 w-5" />,
            label: 'New Indexer'
        },
        {
            path: '/dashboard/db-credentials/add',
            icon: <Database className="h-5 w-5" />,
            label: 'New Credential'
        },
    ];

    const toggleCollapsed = React.useCallback(() => {
        setCollapsed((prev: boolean) => !prev);
    }, []);

    return (
        <motion.aside
            className="bg-background dark:bg-[#171717] border-r border-border flex flex-col relative h-screen z-30"
            initial={false}
            animate={collapsed ? "collapsed" : "expanded"}
            variants={sidebarVariants}
            transition={{ duration: 0.3, ease: "easeInOut" }}
        >
            <Button
                variant="outline"
                size="icon"
                className="absolute -right-3 top-20 bg-black/80 border-gray-800 shadow-lg rounded-full z-10 text-green-500 hover:text-white"
                onClick={toggleCollapsed}
            >
                {collapsed ? <ChevronRight className="h-4 w-4" /> : <ChevronLeft className="h-4 w-4" />}
            </Button>

            <div className="p-4 flex-shrink-0">
                <div className="flex items-center mb-8 justify-between">
                    <Link
                        to="/dashboard"
                        className="flex items-center hover:opacity-80 transition-opacity"
                    >
                        <div className="text-green-500">
                            <LogoIcon />
                        </div>
                        <AnimatePresence>
                            {!collapsed && (
                                <motion.span
                                    className="ml-2 text-xl font-bold whitespace-nowrap overflow-hidden text-black dark:text-white"
                                    initial={{ opacity: 0, width: 0 }}
                                    animate={{ opacity: 1, width: "auto" }}
                                    exit={{ opacity: 0, width: 0 }}
                                    transition={{ duration: 0.3 }}
                                >
                                    Indexer Pro
                                </motion.span>
                            )}
                        </AnimatePresence>
                    </Link>
                    {!collapsed && <ModeToggle />}
                </div>

                <nav className="space-y-1 flex-grow">
                    {navItems.map((item) => (
                        <NavLink
                            key={item.path}
                            to={item.path}
                            end={item.exact}
                            className={({ isActive }) =>
                                `flex items-center p-2 rounded-md transition-all duration-200 ${isActive
                                    ? 'bg-green-500/10 text-green-500 font-medium'
                                    : 'text-primary dark:text-green-500 hover:bg-muted dark:hover:bg-black hover:text-primary dark:hover:text-green-400'
                                }`
                            }
                        >
                            <div className="flex items-center justify-center w-8 h-8">
                                {item.icon}
                            </div>
                            <AnimatePresence>
                                {!collapsed && (
                                    <motion.span
                                        className="ml-3 whitespace-nowrap overflow-hidden"
                                        initial={{ opacity: 0, width: 0 }}
                                        animate={{ opacity: 1, width: "auto" }}
                                        exit={{ opacity: 0, width: 0 }}
                                        transition={{ duration: 0.2 }}
                                    >
                                        {item.label}
                                    </motion.span>
                                )}
                            </AnimatePresence>
                        </NavLink>
                    ))}
                </nav>
            </div>

            {/* Quick actions section */}
            <div className="px-4 py-2 mb-4 mt-2">
                <AnimatePresence>
                    {!collapsed && (
                        <motion.div
                            initial={{ opacity: 0 }}
                            animate={{ opacity: 1 }}
                            exit={{ opacity: 0 }}
                            className="mb-2 px-2 text-xs uppercase font-semibold text-gray-500"
                        >
                            QUICK ACTIONS
                        </motion.div>
                    )}
                </AnimatePresence>
                <div className="space-y-1.5">
                    {quickActions.map((action) => (
                        <NavLink
                            key={action.path}
                            to={action.path}
                            className={({ isActive }) =>
                                `flex items-center p-2 rounded-md transition-all duration-200 text-sm ${isActive
                                    ? 'bg-green-500/10 text-green-500'
                                    : 'text-green-500 hover:bg-black hover:text-green-400'
                                }`
                            }
                        >
                            <div className="flex items-center justify-center w-8 h-8">
                                {action.icon}
                            </div>
                            <AnimatePresence>
                                {!collapsed && (
                                    <motion.span
                                        className="ml-3 whitespace-nowrap overflow-hidden"
                                        initial={{ opacity: 0, width: 0 }}
                                        animate={{ opacity: 1, width: "auto" }}
                                        exit={{ opacity: 0, width: 0 }}
                                        transition={{ duration: 0.2 }}
                                    >
                                        {action.label}
                                    </motion.span>
                                )}
                            </AnimatePresence>
                        </NavLink>
                    ))}
                </div>
            </div>

            {/* Version note section */}
            <AnimatePresence>
                {!collapsed && (
                    <motion.div
                        initial={{ opacity: 0, height: 0 }}
                        animate={{ opacity: 1, height: "auto" }}
                        exit={{ opacity: 0, height: 0 }}
                        transition={{ duration: 0.3 }}
                        className="mx-4 mb-4 p-3 bg-yellow-900/20 border border-yellow-900/30 rounded-lg"
                    >
                        <div className="flex items-start gap-2">
                            <AlertCircle className="h-4 w-4 text-yellow-500 flex-shrink-0 mt-0.5" />
                            <div className="text-xs text-yellow-200">
                                <p className="font-medium">Platform Limitations</p>
                                <p className="mt-1 opacity-90">Currently only 1 indexer can run at a time. Webhooks take 3-4 minutes to push data to your DB.</p>
                                <p className="mt-1 italic">v2 coming soon with unlimited indexers!</p>
                            </div>
                        </div>
                    </motion.div>
                )}
            </AnimatePresence>

            {/* User profile and logout area */}
            <div className="mt-auto border-t border-gray-800 pt-4 pb-6 px-4">
                <div className="flex items-center mb-4 px-2">
                    <div className="w-8 h-8 rounded-full bg-green-500/20 flex items-center justify-center text-green-500">
                        <User className="h-4 w-4" />
                    </div>
                    <AnimatePresence>
                        {!collapsed && (
                            <motion.div
                                className="ml-3 overflow-hidden"
                                initial={{ opacity: 0, width: 0 }}
                                animate={{ opacity: 1, width: "auto" }}
                                exit={{ opacity: 0, width: 0 }}
                                transition={{ duration: 0.2 }}
                            >
                                <div className="font-medium text-sm text-white truncate">
                                    {user?.email || 'User'}
                                </div>
                                <div className="text-xs text-gray-500">
                                    Account Settings
                                </div>
                            </motion.div>
                        )}
                    </AnimatePresence>
                </div>

                <Button
                    variant="destructive"
                    size={collapsed ? "icon" : "default"}
                    className={`${collapsed ? 'w-full aspect-square' : 'w-full'} flex items-center justify-center bg-red-700 hover:bg-red-800`}
                    onClick={handleLogout}
                >
                    <LogOut className={`${collapsed ? 'mr-0' : 'mr-2'} h-4 w-4`} />
                    <AnimatePresence>
                        {!collapsed && (
                            <motion.span
                                initial={{ opacity: 0, width: 0 }}
                                animate={{ opacity: 1, width: "auto" }}
                                exit={{ opacity: 0, width: 0 }}
                                transition={{ duration: 0.2 }}
                            >
                                Logout
                            </motion.span>
                        )}
                    </AnimatePresence>
                </Button>
            </div>
        </motion.aside>
    );
};

export default DashboardSidebar;
