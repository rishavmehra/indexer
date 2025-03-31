import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { AlertCircle, ArrowLeft, RefreshCw } from 'lucide-react';
import { IndexerService } from '@/services/api';

interface IndexerLog {
  id: string;
  indexerId: string;
  eventType: string;
  message: string;
  details: any;
  createdAt: string;
}

interface Indexer {
  id: string;
  indexerType: string;
  targetTable: string;
  status: string;
  webhookId: string;
  lastIndexedAt: string | null;
  errorMessage: string | null;
  createdAt: string;
  updatedAt: string;
}

const IndexerLogs: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const [logs, setLogs] = useState<IndexerLog[]>([]);
  const [indexer, setIndexer] = useState<Indexer | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  const [limit] = useState(100);
  const [offset] = useState(0);
  const navigate = useNavigate();

  const fetchIndexerAndLogs = async () => {
    if (!id) return;
    
    try {
      setIsLoading(true);
      
      // Get indexer details
      const indexerResponse = await IndexerService.getById(id);
      setIndexer(indexerResponse.data);
      
      // Get indexer logs
      const logsResponse = await IndexerService.getLogs(id, limit, offset);
      setLogs(logsResponse.data);
      
      setError('');
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to fetch indexer logs');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchIndexerAndLogs();
  }, [id]);

  const refreshLogs = () => {
    fetchIndexerAndLogs();
  };

  const getEventTypeColor = (eventType: string) => {
    switch (eventType.toLowerCase()) {
      case 'error':
        return 'text-destructive';
      case 'warning':
        return 'text-yellow-500 dark:text-yellow-400';
      case 'success':
        return 'text-green-500 dark:text-green-400';
      case 'initialization':
        return 'text-blue-500 dark:text-blue-400';
      case 'webhook_creation':
        return 'text-purple-500 dark:text-purple-400';
      default:
        return 'text-foreground';
    }
  };

  const formatDateTime = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleString();
  };

  const formatDetails = (details: any) => {
    if (!details) return 'No details';
    
    if (typeof details === 'string') {
      try {
        details = JSON.parse(details);
      } catch (e) {
        return details;
      }
    }
    
    return JSON.stringify(details, null, 2);
  };

  if (isLoading) {
    return (
      <div className="flex justify-center items-center h-32">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div className="flex items-center">
          <Button 
            variant="ghost" 
            onClick={() => navigate('/dashboard/indexers')}
            className="mr-4"
          >
            <ArrowLeft className="h-4 w-4 mr-2" />
            Back to Indexers
          </Button>
          <h2 className="text-2xl font-bold">
            {indexer ? `Logs: ${indexer.targetTable}` : 'Indexer Logs'}
          </h2>
        </div>
        
        <Button 
          onClick={refreshLogs}
          variant="outline"
          className="flex items-center"
        >
          <RefreshCw className="h-4 w-4 mr-2" />
          Refresh
        </Button>
      </div>
      
      {error && (
        <div className="bg-destructive/15 text-destructive text-sm p-3 rounded-md flex items-center">
          <AlertCircle className="h-4 w-4 mr-2" />
          {error}
        </div>
      )}
      
      {logs.length === 0 ? (
        <Card>
          <CardContent className="p-6 text-center">
            <h3 className="text-lg font-medium mb-2">No Logs Available</h3>
            <p className="text-muted-foreground">
              This indexer hasn't generated any logs yet. Logs will appear as the indexer processes data.
            </p>
          </CardContent>
        </Card>
      ) : (
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">Indexing Activity Logs</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead>
                  <tr className="border-b">
                    <th className="text-left py-3 px-4 font-medium">Time</th>
                    <th className="text-left py-3 px-4 font-medium">Event Type</th>
                    <th className="text-left py-3 px-4 font-medium">Message</th>
                    <th className="text-left py-3 px-4 font-medium">Details</th>
                  </tr>
                </thead>
                <tbody>
                  {logs.map((log) => (
                    <tr key={log.id} className="border-b hover:bg-muted/50">
                      <td className="py-3 px-4 text-sm text-muted-foreground">
                        {formatDateTime(log.createdAt)}
                      </td>
                      <td className={`py-3 px-4 text-sm font-medium ${getEventTypeColor(log.eventType)}`}>
                        {log.eventType.charAt(0).toUpperCase() + log.eventType.slice(1)}
                      </td>
                      <td className="py-3 px-4 text-sm">
                        {log.message}
                      </td>
                      <td className="py-3 px-4 text-sm text-muted-foreground">
                        <details>
                          <summary className="cursor-pointer hover:text-primary">View Details</summary>
                          <pre className="mt-2 p-2 bg-muted/50 rounded text-xs whitespace-pre-wrap overflow-x-auto">
                            {formatDetails(log.details)}
                          </pre>
                        </details>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
};

export default IndexerLogs;