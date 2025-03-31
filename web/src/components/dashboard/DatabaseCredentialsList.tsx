import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { AlertCircle, Database, Edit, Trash2, Plus, Copy, CheckCircle, Server, Key, Clock } from 'lucide-react';
import { DBCredentialService } from '@/services/api';
import { motion, AnimatePresence } from 'framer-motion';
import { useToast } from '@/components/ui/toast';

interface DBCredential {
  id: string;
  host: string;
  port: number;
  name: string;
  user: string;
  sslMode: string;
  createdAt: string;
  updatedAt: string;
}

const DatabaseCredentialsList: React.FC = () => {
  const [credentials, setCredentials] = useState<DBCredential[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [copied, setCopied] = useState<string | null>(null);
  const navigate = useNavigate();
  const { addToast } = useToast();

  const fetchCredentials = async () => {
    try {
      setIsLoading(true);
      const response = await DBCredentialService.getAll();
      setCredentials(response.data);
      setError('');
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to fetch database credentials');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchCredentials();
  }, []);

  const handleDelete = async (id: string) => {
    if (deleteId === id) {
      try {
        await DBCredentialService.delete(id);
        setCredentials(prev => prev.filter(cred => cred.id !== id));
        setDeleteId(null);
        addToast({
          title: 'Credential Deleted',
          description: 'Database credential has been deleted successfully.',
          type: 'success'
        });
      } catch (err: any) {
        setError(err.response?.data?.error || 'Failed to delete database credential');
        addToast({
          title: 'Error',
          description: err.response?.data?.error || 'Failed to delete database credential',
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

  const copyToClipboard = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopied(id);
    
    // Clear the copied state after 2 seconds
    setTimeout(() => {
      setCopied(null);
    }, 2000);
  };

  const containerVariants = {
    hidden: { opacity: 0 },
    visible: {
      opacity: 1,
      transition: {
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
    },
    exit: {
      opacity: 0,
      y: -20,
      transition: { duration: 0.2 }
    }
  };

  if (isLoading) {
    return (
      <div className="flex justify-center items-center h-64">
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
          Database Credentials
        </motion.h2>
        <motion.div variants={itemVariants}>
          <Button 
            onClick={() => navigate('/dashboard/db-credentials/add')}
            className="flex items-center gap-2 relative overflow-hidden group"
          >
            <Plus className="h-4 w-4" />
            <span className="relative z-10">Add Credential</span>
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
      
      {credentials.length === 0 ? (
        <motion.div variants={itemVariants}>
          <Card>
            <CardContent className="p-6 text-center">
              <Database className="mx-auto h-12 w-12 text-muted-foreground mb-4" />
              <h3 className="text-lg font-medium mb-2">No Database Credentials</h3>
              <p className="text-muted-foreground mb-4">
                Add your first PostgreSQL database credential to start indexing blockchain data.
              </p>
              <Button 
                onClick={() => navigate('/dashboard/db-credentials/add')}
                className="mx-auto relative overflow-hidden group"
              >
                <span className="relative z-10">Add Your First Credential</span>
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
            {credentials.map(credential => (
              <motion.div 
                key={credential.id}
                variants={itemVariants}
                exit="exit"
                layout
                whileHover={{ scale: 1.02, transition: { duration: 0.2 } }}
              >
                <Card className="overflow-hidden border-border/60 hover:shadow-md transition-all duration-300 hover:border-primary/30 bg-card/80">
                  <CardHeader className="bg-primary/10 pb-3 pt-4">
                    <CardTitle className="flex justify-between items-center text-base">
                      <div className="flex items-center gap-2 truncate">
                        <Database className="h-5 w-5 text-primary" />
                        <span className="truncate">{credential.name}</span>
                      </div>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8"
                        onClick={() => copyToClipboard(`${credential.host}:${credential.port}/${credential.name}`, `connection-${credential.id}`)}
                        title="Copy connection string"
                      >
                        {copied === `connection-${credential.id}` ? (
                          <CheckCircle className="h-4 w-4 text-green-500" />
                        ) : (
                          <Copy className="h-4 w-4" />
                        )}
                      </Button>
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="p-4 space-y-3">
                    <div className="flex items-center gap-2 text-sm">
                      <Server className="h-4 w-4 text-muted-foreground" />
                      <div className="flex justify-between w-full items-center">
                        <span className="text-muted-foreground">Host:</span>
                        <div className="flex items-center gap-1 font-medium">
                          <span className="truncate max-w-[140px]">{credential.host}</span>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-6 w-6"
                            onClick={() => copyToClipboard(credential.host, `host-${credential.id}`)}
                          >
                            {copied === `host-${credential.id}` ? (
                              <CheckCircle className="h-3 w-3 text-green-500" />
                            ) : (
                              <Copy className="h-3 w-3" />
                            )}
                          </Button>
                        </div>
                      </div>
                    </div>
                    
                    <div className="flex items-center gap-2 text-sm">
                      <Database className="h-4 w-4 text-muted-foreground" />
                      <div className="flex justify-between w-full items-center">
                        <span className="text-muted-foreground">Port:</span>
                        <span className="font-medium">{credential.port}</span>
                      </div>
                    </div>
                    
                    <div className="flex items-center gap-2 text-sm">
                      <Key className="h-4 w-4 text-muted-foreground" />
                      <div className="flex justify-between w-full items-center">
                        <span className="text-muted-foreground">User:</span>
                        <div className="flex items-center gap-1 font-medium">
                          <span>{credential.user}</span>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-6 w-6"
                            onClick={() => copyToClipboard(credential.user, `user-${credential.id}`)}
                          >
                            {copied === `user-${credential.id}` ? (
                              <CheckCircle className="h-3 w-3 text-green-500" />
                            ) : (
                              <Copy className="h-3 w-3" />
                            )}
                          </Button>
                        </div>
                      </div>
                    </div>
                    
                    <div className="flex items-center gap-2 text-sm">
                      <Clock className="h-4 w-4 text-muted-foreground" />
                      <div className="flex justify-between w-full items-center">
                        <span className="text-muted-foreground">Created:</span>
                        <span className="font-medium">{new Date(credential.createdAt).toLocaleDateString()}</span>
                      </div>
                    </div>
                    
                    <div className="pt-3 flex justify-end space-x-2">
                      <Button
                        variant="outline"
                        size="sm"
                        className="text-sm"
                        onClick={() => navigate(`/dashboard/db-credentials/edit/${credential.id}`)}
                      >
                        <Edit className="h-3.5 w-3.5 mr-1" />
                        Edit
                      </Button>
                      <Button
                        variant={deleteId === credential.id ? "destructive" : "outline"}
                        size="sm"
                        className="text-sm"
                        onClick={() => handleDelete(credential.id)}
                      >
                        <Trash2 className="h-3.5 w-3.5 mr-1" />
                        {deleteId === credential.id ? 'Confirm' : 'Delete'}
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

export default DatabaseCredentialsList;

