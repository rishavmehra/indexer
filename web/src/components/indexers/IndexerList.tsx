import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { AlertCircle, BarChart3, Play, Pause, Trash2, Plus, FileText } from 'lucide-react';
import { IndexerService } from '@/services/api';

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
}

const IndexerList: React.FC = () => {
  const [indexers, setIndexers] = useState<Indexer[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const navigate = useNavigate();

  const fetchIndexers = async () => {
    try {
      setIsLoading(true);
      const response = await IndexerService.getAll();
      setIndexers(response.data);
      setError('');
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to fetch indexers');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchIndexers();
  }, []);

  const handleDelete = async (id: string) => {
    if (deleteId === id) {
      try {
        await IndexerService.delete(id);
        setIndexers(prev => prev.filter(indexer => indexer.id !== id));
        setDeleteId(null);
      } catch (err: any) {
        setError(err.response?.data?.error || 'Failed to delete indexer');
      }
    } else {
      setDeleteId(id);
    }
  };

  const handlePauseResume = async (id: string, currentStatus: string) => {
    try {
      if (currentStatus === 'active') {
        await IndexerService.pause(id);
      } else if (currentStatus === 'paused') {
        await IndexerService.resume(id);
      }
      // Refresh the list after action
      fetchIndexers();
    } catch (err: any) {
      setError(err.response?.data?.error || `Failed to ${currentStatus === 'active' ? 'pause' : 'resume'} indexer`);
    }
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

  if (isLoading) {
    return (
      <div className="flex justify-center items-center h-32">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <h2 className="text-2xl font-bold">Blockchain Indexers</h2>
        <Button 
          onClick={() => navigate('/dashboard/indexers/create')}
          className="flex items-center"
        >
          <Plus className="mr-2 h-4 w-4" />
          Create Indexer
        </Button>
      </div>
      
      {error && (
        <div className="bg-destructive/15 text-destructive text-sm p-3 rounded-md flex items-center mb-4">
          <AlertCircle className="h-4 w-4 mr-2" />
          {error}
        </div>
      )}
      
      {indexers.length === 0 ? (
        <Card>
          <CardContent className="p-6 text-center">
            <BarChart3 className="mx-auto h-12 w-12 text-muted-foreground mb-4" />
            <h3 className="text-lg font-medium mb-2">No Indexers Created</h3>
            <p className="text-muted-foreground mb-4">
              Create your first indexer to start collecting blockchain data in your database.
            </p>
            <Button 
              onClick={() => navigate('/dashboard/indexers/create')}
              className="mx-auto"
            >
              Create Your First Indexer
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {indexers.map(indexer => (
            <Card key={indexer.id} className="overflow-hidden">
              <div className="bg-primary/10 p-4 flex justify-between items-center">
                <div className="flex items-center">
                  <BarChart3 className="h-5 w-5 mr-2" />
                  <h3 className="font-medium truncate">{indexer.targetTable}</h3>
                </div>
                <span className={`px-2 py-1 rounded-full text-xs font-medium ${getStatusBadgeColor(indexer.status)}`}>
                  {indexer.status.charAt(0).toUpperCase() + indexer.status.slice(1)}
                </span>
              </div>
              <CardContent className="p-4 space-y-2">
                <div className="text-sm">
                  <span className="text-muted-foreground">Type:</span> {getIndexerTypeLabel(indexer.indexerType)}
                </div>
                {indexer.lastIndexedAt && (
                  <div className="text-sm">
                    <span className="text-muted-foreground">Last Indexed:</span> {new Date(indexer.lastIndexedAt).toLocaleString()}
                  </div>
                )}
                {indexer.errorMessage && (
                  <div className="text-sm text-destructive truncate" title={indexer.errorMessage}>
                    <span className="text-destructive font-medium">Error:</span> {indexer.errorMessage}
                  </div>
                )}
                <div className="pt-2 flex justify-end space-x-2">
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => navigate(`/dashboard/indexers/${indexer.id}/logs`)}
                    title="View Logs"
                  >
                    <FileText className="h-4 w-4 mr-1" />
                    Logs
                  </Button>
                  
                  {indexer.status === 'active' ? (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => handlePauseResume(indexer.id, indexer.status)}
                      title="Pause Indexer"
                    >
                      <Pause className="h-4 w-4 mr-1" />
                      Pause
                    </Button>
                  ) : indexer.status === 'paused' ? (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => handlePauseResume(indexer.id, indexer.status)}
                      title="Resume Indexer"
                    >
                      <Play className="h-4 w-4 mr-1" />
                      Resume
                    </Button>
                  ) : null}
                  
                  <Button
                    variant={deleteId === indexer.id ? "destructive" : "ghost"}
                    size="sm"
                    onClick={() => handleDelete(indexer.id)}
                  >
                    <Trash2 className="h-4 w-4 mr-1" />
                    {deleteId === indexer.id ? 'Confirm' : 'Delete'}
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
};

export default IndexerList;