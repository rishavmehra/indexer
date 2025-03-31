import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Database, BarChart3, Settings, Users, CheckCircle, ExternalLink, PlusCircle, Activity } from 'lucide-react';
import { UserService, DBCredentialService, IndexerService } from '@/services/api';
import { motion } from 'framer-motion';
import { Button } from '@/components/ui/button';

interface UserInfo {
    id: string;
    email: string;
    createdAt: string;
}

const DashboardPage: React.FC = () => {
    const [user, setUser] = useState<UserInfo | null>(null);
    const [dbCredCount, setDbCredCount] = useState<number>(0);
    const [indexerCount, setIndexerCount] = useState<number>(0);
    const [activeIndexerCount, setActiveIndexerCount] = useState<number>(0);
    const [isLoading, setIsLoading] = useState(true);

    useEffect(() => {
        const fetchDashboardData = async () => {
            try {
                setIsLoading(true);

                // Fetch user info
                const userResponse = await UserService.getCurrentUser();
                setUser(userResponse.data);

                // Fetch database credentials count
                const dbCredResponse = await DBCredentialService.getAll();
                setDbCredCount(dbCredResponse.data.length);
                
                // Fetch indexers count
                try {
                    const indexerResponse = await IndexerService.getAll();
                    const indexers = indexerResponse.data;
                    setIndexerCount(indexers.length);
                    setActiveIndexerCount(indexers.filter((i: any) => i.status === 'active').length);
                } catch (error) {
                    console.error('Failed to fetch indexers', error);
                    setIndexerCount(0);
                    setActiveIndexerCount(0);
                }
            } catch (error) {
                console.error('Failed to fetch dashboard data', error);
            } finally {
                setIsLoading(false);
            }
        };

        fetchDashboardData();
    }, []);

    const containerVariants = {
        hidden: { opacity: 0 },
        visible: {
            opacity: 1,
            transition: {
                duration: 0.3,
                when: "beforeChildren",
                staggerChildren: 0.1
            }
        }
    };

    const itemVariants = {
        hidden: { opacity: 0, y: 20 },
        visible: {
            opacity: 1,
            y: 0,
            transition: { duration: 0.3 }
        }
    };

    const hoverAnimation = {
        scale: 1.02,
        transition: { duration: 0.2 }
    };

    if (isLoading) {
        return (
            <div className="flex justify-center items-center h-full min-h-[400px]">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
            </div>
        );
    }

    return (
        <motion.div 
            className="p-6 max-w-7xl mx-auto"
            initial="hidden"
            animate="visible"
            variants={containerVariants}
        >
            <motion.div 
                className="flex justify-between items-center mb-6"
                variants={itemVariants}
            >
                <div>
                    <h1 className="text-3xl font-bold">Dashboard</h1>
                    {user && (
                        <p className="text-muted-foreground mt-1">
                            Welcome back, <span className="text-primary font-medium">{user.email}</span>
                        </p>
                    )}
                </div>
                <div className="flex gap-3">
                    <Button 
                        variant="outline" 
                        className="gap-2"
                        onClick={() => window.location.reload()}
                    >
                        <Activity className="h-4 w-4" />
                        Refresh
                    </Button>
                    <Button asChild>
                        <Link to="/dashboard/indexers/create" className="gap-2">
                            <PlusCircle className="h-4 w-4" />
                            New Indexer
                        </Link>
                    </Button>
                </div>
            </motion.div>

            <motion.div 
                className="grid gap-6 md:grid-cols-2 lg:grid-cols-4"
                variants={containerVariants}
            >
                <motion.div 
                    variants={itemVariants} 
                    whileHover={hoverAnimation}
                >
                    <Link to="/dashboard/db-credentials" className="block h-full">
                        <Card className="h-full border-border/60 hover:shadow-md transition-all duration-300 hover:border-primary/30 bg-card/70 group">
                            <CardHeader className="flex flex-row items-center justify-between pb-2">
                                <CardTitle className="text-sm font-medium flex items-center gap-2">
                                    <Database className="h-4 w-4 text-primary" />
                                    Database Credentials
                                </CardTitle>
                                <div className="h-8 w-8 rounded-full bg-primary/10 flex items-center justify-center group-hover:bg-primary/20 transition-colors duration-300">
                                    <span className="text-xs font-semibold text-primary">{dbCredCount}</span>
                                </div>
                            </CardHeader>
                            <CardContent>
                                <div className="text-2xl font-bold text-foreground">{dbCredCount}</div>
                                <p className="text-xs text-muted-foreground mt-1">
                                    {dbCredCount === 0
                                        ? "Add your first database credential"
                                        : `You have ${dbCredCount} database ${dbCredCount === 1 ? 'credential' : 'credentials'}`}
                                </p>
                                <div className="mt-4 opacity-0 group-hover:opacity-100 transition-opacity">
                                    <Button variant="ghost" size="sm" className="text-xs w-full" asChild>
                                        <Link to="/dashboard/db-credentials">
                                            View All
                                            <ExternalLink className="ml-2 h-3 w-3" />
                                        </Link>
                                    </Button>
                                </div>
                            </CardContent>
                        </Card>
                    </Link>
                </motion.div>

                <motion.div 
                    variants={itemVariants} 
                    whileHover={hoverAnimation}
                >
                    <Link to="/dashboard/indexers" className="block h-full">
                        <Card className="h-full border-border/60 hover:shadow-md transition-all duration-300 hover:border-primary/30 bg-card/70 group">
                            <CardHeader className="flex flex-row items-center justify-between pb-2">
                                <CardTitle className="text-sm font-medium flex items-center gap-2">
                                    <BarChart3 className="h-4 w-4 text-primary" />
                                    Blockchain Indexers
                                </CardTitle>
                                <div className="h-8 w-8 rounded-full bg-primary/10 flex items-center justify-center group-hover:bg-primary/20 transition-colors duration-300">
                                    <span className="text-xs font-semibold text-primary">{indexerCount}</span>
                                </div>
                            </CardHeader>
                            <CardContent>
                                <div className="text-2xl font-bold">
                                    {indexerCount}
                                    {activeIndexerCount > 0 && (
                                        <span className="ml-2 text-sm font-normal text-green-500">
                                            ({activeIndexerCount} active)
                                        </span>
                                    )}
                                </div>
                                <p className="text-xs text-muted-foreground">
                                    {indexerCount === 0
                                        ? "Create your first blockchain indexer"
                                        : `You have ${indexerCount} ${indexerCount === 1 ? 'indexer' : 'indexers'}`}
                                </p>
                                <div className="mt-4 opacity-0 group-hover:opacity-100 transition-opacity">
                                    <Button variant="ghost" size="sm" className="text-xs w-full" asChild>
                                        <Link to="/dashboard/indexers">
                                            View All
                                            <ExternalLink className="ml-2 h-3 w-3" />
                                        </Link>
                                    </Button>
                                </div>
                            </CardContent>
                        </Card>
                    </Link>
                </motion.div>

                <motion.div 
                    variants={itemVariants} 
                    whileHover={hoverAnimation}
                >
                    <Card className="h-full border-border/60 hover:shadow-md transition-all duration-300 hover:border-primary/30 bg-card/70 group">
                        <CardHeader className="flex flex-row items-center justify-between pb-2">
                            <CardTitle className="text-sm font-medium flex items-center gap-2">
                                <Users className="h-4 w-4 text-primary" />
                                Account Status
                            </CardTitle>
                            <div className="h-8 w-8 rounded-full bg-green-100 dark:bg-green-900/30 flex items-center justify-center">
                                <CheckCircle className="h-4 w-4 text-green-600 dark:text-green-400" />
                            </div>
                        </CardHeader>
                        <CardContent>
                            <div className="flex items-center gap-1">
                                <div className="text-2xl font-bold">Active</div>
                            </div>
                            <p className="text-xs text-muted-foreground">
                                Account created {user && new Date(user.createdAt).toLocaleDateString()}
                            </p>
                            <div className="mt-4 opacity-0 group-hover:opacity-100 transition-opacity">
                                <Button variant="ghost" size="sm" className="text-xs w-full" asChild>
                                    <Link to="/dashboard/settings">
                                        Settings
                                        <Settings className="ml-2 h-3 w-3" />
                                    </Link>
                                </Button>
                            </div>
                        </CardContent>
                    </Card>
                </motion.div>

                <motion.div 
                    variants={itemVariants} 
                    whileHover={hoverAnimation}
                >
                    <Card className="h-full border-border/60 hover:shadow-md transition-all duration-300 hover:border-primary/30 bg-card/70 group">
                        <CardHeader className="flex flex-row items-center justify-between pb-2">
                            <CardTitle className="text-sm font-medium flex items-center gap-2">
                                <Activity className="h-4 w-4 text-primary" />
                                System Status
                            </CardTitle>
                            <div className="h-8 w-8 rounded-full bg-green-100 dark:bg-green-900/30 flex items-center justify-center">
                                <CheckCircle className="h-4 w-4 text-green-600 dark:text-green-400" />
                            </div>
                        </CardHeader>
                        <CardContent>
                            <div className="text-2xl font-bold">
                                Online
                            </div>
                            <p className="text-xs text-muted-foreground">
                                All systems operational
                            </p>
                            <div className="mt-4 opacity-0 group-hover:opacity-100 transition-opacity">
                                <Button variant="ghost" size="sm" className="text-xs w-full" asChild>
                                    <Link to="#">
                                        System Info
                                        <ExternalLink className="ml-2 h-3 w-3" />
                                    </Link>
                                </Button>
                            </div>
                        </CardContent>
                    </Card>
                </motion.div>
            </motion.div>

            <motion.div 
                className="mt-8"
                variants={itemVariants}
            >
                <div className="flex items-center gap-2 mb-4">
                    <h2 className="text-xl font-semibold">Getting Started</h2>
                    <div className="h-1 w-24 bg-gradient-to-r from-primary/40 to-primary rounded-full"></div>
                </div>
                <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
                    <motion.div 
                        variants={itemVariants}
                        whileHover={{ y: -5, transition: { duration: 0.2 } }}
                    >
                        <Link to="/dashboard/db-credentials/add" className="block">
                            <Card className="hover:shadow-lg transition-all duration-300 hover:border-primary/30 cursor-pointer group bg-background/70 hover:bg-accent/20 border border-border/60">
                                <CardHeader>
                                    <CardTitle className="text-lg group-hover:text-primary transition-colors flex items-center gap-2">
                                        <Database className="h-5 w-5 group-hover:text-primary text-muted-foreground transition-colors" />
                                        Add Database Credentials
                                    </CardTitle>
                                </CardHeader>
                                <CardContent>
                                    <p className="text-sm text-muted-foreground group-hover:text-foreground transition-colors">
                                        Connect your PostgreSQL database to start indexing blockchain data. You'll need your
                                        database host, port, name, username, and password.
                                    </p>
                                    <div className="mt-4 flex justify-end">
                                        <ExternalLink className="h-4 w-4 text-muted-foreground group-hover:text-primary transition-colors" />
                                    </div>
                                </CardContent>
                            </Card>
                        </Link>
                    </motion.div>

                    <motion.div 
                        variants={itemVariants}
                        whileHover={{ y: -5, transition: { duration: 0.2 } }}
                    >
                        <Link to="/dashboard/indexers/create" className="block">
                            <Card className="hover:shadow-lg transition-all duration-300 hover:border-primary/30 cursor-pointer group bg-background/70 hover:bg-accent/20 border border-border/60">
                                <CardHeader>
                                    <CardTitle className="text-lg group-hover:text-primary transition-colors flex items-center gap-2">
                                        <BarChart3 className="h-5 w-5 group-hover:text-primary text-muted-foreground transition-colors" />
                                        Create Blockchain Indexer
                                    </CardTitle>
                                </CardHeader>
                                <CardContent>
                                    <p className="text-sm text-muted-foreground group-hover:text-foreground transition-colors">
                                        Choose what blockchain data you want to index, such as NFT prices, bids, token prices,
                                        or borrowing availability.
                                    </p>
                                    <div className="mt-4 flex justify-end">
                                        <ExternalLink className="h-4 w-4 text-muted-foreground group-hover:text-primary transition-colors" />
                                    </div>
                                </CardContent>
                            </Card>
                        </Link>
                    </motion.div>

                    <motion.div 
                        variants={itemVariants}
                        whileHover={{ y: -5, transition: { duration: 0.2 } }}
                    >
                        <Link to="/dashboard/indexers" className="block">
                            <Card className="hover:shadow-lg transition-all duration-300 hover:border-primary/30 cursor-pointer group bg-background/70 hover:bg-accent/20 border border-border/60">
                                <CardHeader>
                                    <CardTitle className="text-lg group-hover:text-primary transition-colors flex items-center gap-2">
                                        <Settings className="h-5 w-5 group-hover:text-primary text-muted-foreground transition-colors" />
                                        Monitor Your Indexers
                                    </CardTitle>
                                </CardHeader>
                                <CardContent>
                                    <p className="text-sm text-muted-foreground group-hover:text-foreground transition-colors">
                                        View logs, manage indexers, and ensure your blockchain data is successfully synchronized 
                                        to your database in real-time.
                                    </p>
                                    <div className="mt-4 flex justify-end">
                                        <ExternalLink className="h-4 w-4 text-muted-foreground group-hover:text-primary transition-colors" />
                                    </div>
                                </CardContent>
                            </Card>
                        </Link>
                    </motion.div>
                </div>
            </motion.div>
        </motion.div>
    );
};

export default DashboardPage;
