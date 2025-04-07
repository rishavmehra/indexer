import React, { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import {
  AlertCircle,
  BarChart3,
  Play,
  Pause,
  Trash2,
  Plus,
  FileText,
  Server,
  Database,
  Clock
} from 'lucide-react';
import { IndexerService, DBCredentialService } from '@/services/api';
import { motion, AnimatePresence } from 'framer-motion';
import { useToast } from '@/components/ui/toast';

interface Indexer {
  id: string;
  indexerType: string;
  targetTable: string;
  status: 'pending' | 'active' | 'paused' | 'failed' | 'completed';
  webhookId: string;
  lastIndexedAt: string | null;
  errorMessage: string | null;
  createdAt: string;
  updatedAt: string;
  params: any;
  dbCredential: {
    id: string;
    host: string;
    port: number;
    name: string;
    user: string;
  } | null;
  dbCredentialId: string;
}

interface DBCredential {
  id: string;
  host: string;
  port: number;
  name: string;
  user: string;
}

const IndexerList: React.FC = () => {
  const [indexers, setIndexers] = useState<Indexer[]>([]);
  const [credentials, setCredentials] = useState<{[key: string]: DBCredential}>({});
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const navigate = useNavigate();
  const { addToast } = useToast();

  // Fetch all database credentials to ensure we have them even if relations aren't loaded properly
  const fetchCredentials = useCallback(async () => {
    try {
      const response = await DBCredentialService.getAll();
      const credMap: {[key: string]: DBCredential} = {};
      response.data.forEach((cred: DBCredential) => {
        credMap[cred.id] = cred;
      });
      setCredentials(credMap);
    } catch (err) {
      console.error('Failed to fetch credentials', err);
    }
  }, []);

  const fetchIndexers = async () => {
    try {
      setIsLoading(true);
      const response = await IndexerService.getAll();
      setIndexers(response.data);
      setError('');
    } catch (err: any) {
      const errorMessage = err.response?.data?.error || 'Failed to fetch indexers';
      setError(errorMessage);
      addToast({
        title: 'Error',
        description: errorMessage,
        type: 'error'
      });
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    const loadData = async () => {
      await fetchCredentials();
      await fetchIndexers();
    };
    
    loadData();
  }, [fetchCredentials]);

  const handleDelete = async (id: string) => {
    if (deleteId === id) {
      try {
        await IndexerService.delete(id);
        setIndexers(prev => prev.filter(indexer => indexer.id !== id));
        setDeleteId(null);
        addToast({
          title: 'Indexer Deleted',
          description: 'Indexer has been deleted successfully.',
          type: 'success'
        });
      } catch (err: any) {
        const errorMessage = err.response?.data?.error || 'Failed to delete indexer';
        setError(errorMessage);
        addToast({
          title: 'Error',
          description: errorMessage,
          type: 'error'
        });
      }
    } else {
      setDeleteId(id);
      // Auto-reset delete confirmation after 5 seconds
      setTimeout(() => {
        setDeleteId(null);
      }, 5000);
    }
  };

  const handlePauseResume = async (id: string, currentStatus: string) => {
    try {
      if (currentStatus === 'active') {
        await IndexerService.pause(id);
        addToast({
          title: 'Indexer Paused',
          description: 'Indexer has been paused successfully.',
          type: 'success'
        });
      } else if (currentStatus === 'paused') {
        await IndexerService.resume(id);
        addToast({
          title: 'Indexer Resumed',
          description: 'Indexer has been resumed successfully.',
          type: 'success'
        });
      }
      // Refresh the list after action
      fetchIndexers();
    } catch (err: any) {
      const errorMessage = err.response?.data?.error || `Failed to ${currentStatus === 'active' ? 'pause' : 'resume'} indexer`;
      setError(errorMessage);
      addToast({
        title: 'Error',
        description: errorMessage,
        type: 'error'
      });
    }
  };

  // Helper function to get database info for display
  const getDatabaseInfo = (indexer: Indexer) => {
    // First try to use the directly associated dbCredential object
    if (indexer.dbCredential) {
      return `${indexer.dbCredential.host}:${indexer.dbCredential.port}/${indexer.dbCredential.name}`;
    }
    
    // If dbCredential is not loaded but we have the ID, try to use our fetched credentials map
    if (indexer.dbCredentialId && credentials[indexer.dbCredentialId]) {
      const cred = credentials[indexer.dbCredentialId];
      return `${cred.host}:${cred.port}/${cred.name}`;
    }
    
    // Fallback to Unknown if we couldn't find the credential
    return 'Unknown Database';
  };

  const getIndexerTypeLabel = (type: string) => {
    switch (type) {
      case 'nft_bids':
        return 'NFT Bids';
      case 'nft_prices':
        return 'NFT Prices';
      case 'token_borrow':
        return 'Token Borrowing';
      case 'token_prices':
        return 'Token Prices';
      default:
        return type;
    }
  };

  const getStatusBadgeColor = (status: string) => {
    switch (status) {
      case 'active':
        return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400';
      case 'paused':
        return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400';
      case 'failed':
        return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400';
      case 'completed':
        return 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400';
      default: // pending
        return 'bg-gray-100 text-gray-800 dark:bg-gray-900/30 dark:text-gray-400';
    }
  };

  const containerVariants = {
    hidden: { opacity: 0 },
    visible: {
      opacity: 1,
      transition: {
        staggerChildren: 0.1,
        delayChildren: 0.1
      }
    }
  };

  const itemVariants = {
    hidden: { opacity: 0, y: 20 },
    visible: {
      opacity: 1,
      y: 0,
      transition: { duration: 0.3 }
    },
    exit: {
      opacity: 0,
      y: -20,
      transition: { duration: 0.2 }
    }
  };

  if (isLoading) {
    return (
      <div className="flex justify-center items-center h-32">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }

  return (
    <motion.div
      initial="hidden"
      animate="visible"
      variants={containerVariants}
    >
      <div className="flex justify-between items-center mb-6">
        <motion.h2
          className="text-2xl font-bold"
          variants={itemVariants}
        >
          Blockchain Indexers
        </motion.h2>
        <motion.div variants={itemVariants}>
          <Button
            onClick={() => navigate('/dashboard/indexers/create')}
            className="flex items-center gap-2 relative overflow-hidden group"
          >
            <Plus className="h-4 w-4" />
            <span className="relative z-10">Create Indexer</span>
            <span className="absolute inset-0 bg-gradient-to-r from-primary/80 to-primary opacity-0 group-hover:opacity-100 transition-opacity duration-300"></span>
          </Button>
        </motion.div>
      </div>

      <AnimatePresence>
        {error && (
          <motion.div
            className="bg-destructive/15 text-destructive text-sm p-3 rounded-md flex items-center mb-4"
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: 'auto' }}
            exit={{ opacity: 0, height: 0 }}
          >
            <AlertCircle className="h-4 w-4 mr-2 flex-shrink-0" />
            {error}
          </motion.div>
        )}
      </AnimatePresence>

      {indexers.length === 0 ? (
        <motion.div variants={itemVariants}>
          <Card>
            <CardContent className="p-6 text-center">
              <BarChart3 className="mx-auto h-12 w-12 text-muted-foreground mb-4" />
              <h3 className="text-lg font-medium mb-2">No Indexers Created</h3>
              <p className="text-muted-foreground mb-4">
                Create your first indexer to start collecting blockchain data in your database.
              </p>
              <Button
                onClick={() => navigate('/dashboard/indexers/create')}
                className="mx-auto relative overflow-hidden group"
              >
                <span className="relative z-10">Create Your First Indexer</span>
                <span className="absolute inset-0 bg-gradient-to-r from-primary/80 to-primary opacity-0 group-hover:opacity-100 transition-opacity duration-300"></span>
              </Button>
            </CardContent>
          </Card>
        </motion.div>
      ) : (
        <motion.div
          className="grid gap-4 md:grid-cols-2 lg:grid-cols-3"
          variants={containerVariants}
        >
          <AnimatePresence>
            {indexers.map(indexer => (
              <motion.div
                key={indexer.id}
                variants={itemVariants}
                exit="exit"
                layout
                whileHover={{ scale: 1.02, transition: { duration: 0.2 } }}
              >
                <Card className="overflow-hidden border-border/60 hover:shadow-md transition-all duration-300 hover:border-primary/30 bg-card/80">
                  <div className="bg-primary/10 p-4 flex justify-between items-center">
                    <div className="flex items-center gap-2 truncate">
                      <BarChart3 className="h-5 w-5 text-primary" />
                      <span className="font-medium truncate">{indexer.targetTable}</span>
                    </div>
                    <span className={`px-2 py-1 rounded-full text-xs font-medium ${getStatusBadgeColor(indexer.status)}`}>
                      {indexer.status.charAt(0).toUpperCase() + indexer.status.slice(1)}
                    </span>
                  </div>
                  <CardContent className="p-4 space-y-3">
                    <div className="flex items-center gap-2 text-sm">
                      <Database className="h-4 w-4 text-muted-foreground" />
                      <div className="flex justify-between w-full items-center">
                        <span className="text-muted-foreground">Type:</span>
                        <span className="font-medium">{getIndexerTypeLabel(indexer.indexerType)}</span>
                      </div>
                    </div>

                    <div className="flex items-center gap-2 text-sm">
                      <Server className="h-4 w-4 text-muted-foreground" />
                      <div className="flex justify-between w-full items-center">
                        <span className="text-muted-foreground">Database:</span>
                        <span className="font-medium truncate max-w-[180px]" title={getDatabaseInfo(indexer)}>
                          {getDatabaseInfo(indexer)}
                        </span>
                      </div>
                    </div>

                    {indexer.lastIndexedAt && (
                      <div className="flex items-center gap-2 text-sm">
                        <Clock className="h-4 w-4 text-muted-foreground" />
                        <div className="flex justify-between w-full items-center">
                          <span className="text-muted-foreground">Last Indexed:</span>
                          <span className="font-medium">{new Date(indexer.lastIndexedAt).toLocaleString()}</span>
                        </div>
                      </div>
                    )}

                    {indexer.errorMessage && (
                      <div className="text-sm text-destructive truncate" title={indexer.errorMessage}>
                        <span className="text-destructive font-medium">Error:</span> {indexer.errorMessage}
                      </div>
                    )}

                    <div className="pt-3 flex justify-end space-x-2">
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-sm"
                        onClick={() => navigate(`/dashboard/indexers/${indexer.id}/logs`)}
                      >
                        <FileText className="h-3.5 w-3.5 mr-1" />
                        Logs
                      </Button>

                      {indexer.status === 'active' ? (
                        <Button
                          variant="outline"
                          size="sm"
                          className="text-sm"
                          onClick={() => handlePauseResume(indexer.id, indexer.status)}
                        >
                          <Pause className="h-3.5 w-3.5 mr-1" />
                          Pause
                        </Button>
                      ) : indexer.status === 'paused' ? (
                        <Button
                          variant="outline"
                          size="sm"
                          className="text-sm"
                          onClick={() => handlePauseResume(indexer.id, indexer.status)}
                        >
                          <Play className="h-3.5 w-3.5 mr-1" />
                          Resume
                        </Button>
                      ) : null}

                      <Button
                        variant={deleteId === indexer.id ? "destructive" : "outline"}
                        size="sm"
                        className="text-sm"
                        onClick={() => handleDelete(indexer.id)}
                      >
                        <Trash2 className="h-3.5 w-3.5 mr-1" />
                        {deleteId === indexer.id ? 'Confirm' : 'Delete'}
                      </Button>
                    </div>
                  </CardContent>
                </Card>
              </motion.div>
            ))}
          </AnimatePresence>
        </motion.div>
      )}
    </motion.div>
  );
};

export default IndexerList;
