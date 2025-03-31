import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card, CardContent, CardFooter, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { AlertCircle, Info } from 'lucide-react';
import { IndexerService, DBCredentialService } from '@/services/api';

interface DBCredential {
  id: string;
  host: string;
  port: number;
  name: string;
  user: string;
}

// Define specific interfaces for each indexer type's parameters
interface NFTParams {
  collection: string;
  marketplaces: string;
}

interface TokenParams {
  tokens: string;
  platforms: string;
}

// Create a map type for the parameters
interface ParamsFieldsMap {
  nft_bids: NFTParams;
  nft_prices: NFTParams;
  token_borrow: TokenParams;
  token_prices: TokenParams;
}

type IndexerType = keyof ParamsFieldsMap;

const IndexerForm: React.FC = () => {
  const [indexerType, setIndexerType] = useState<IndexerType>('nft_bids');
  const [targetTable, setTargetTable] = useState('');
  const [credentials, setCredentials] = useState<DBCredential[]>([]);
  const [selectedCredential, setSelectedCredential] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [loadingCredentials, setLoadingCredentials] = useState(true);
  const [error, setError] = useState('');
  const [paramsFields, setParamsFields] = useState<ParamsFieldsMap>({
    nft_bids: {
      collection: '',
      marketplaces: '',
    },
    nft_prices: {
      collection: '',
      marketplaces: '',
    },
    token_borrow: {
      tokens: '',
      platforms: '',
    },
    token_prices: {
      tokens: '',
      platforms: '',
    },
  });
  
  const navigate = useNavigate();

  useEffect(() => {
    const fetchCredentials = async () => {
      try {
        setLoadingCredentials(true);
        const response = await DBCredentialService.getAll();
        setCredentials(response.data);
        if (response.data.length > 0) {
          setSelectedCredential(response.data[0].id);
        }
      } catch (err: any) {
        setError(err.response?.data?.error || 'Failed to fetch database credentials');
      } finally {
        setLoadingCredentials(false);
      }
    };

    fetchCredentials();
  }, []);

  const handleParamsChange = (
    type: IndexerType,
    field: string,
    value: string
  ) => {
    setParamsFields(prev => ({
      ...prev,
      [type]: {
        ...prev[type],
        [field]: value
      }
    }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError('');

    if (!selectedCredential) {
      setError('Please select a database credential');
      setIsLoading(false);
      return;
    }

    if (!targetTable) {
      setError('Please enter a target table name');
      setIsLoading(false);
      return;
    }

    // Prepare the parameters based on indexer type
    let params: any = {};
    
    if (indexerType === 'nft_bids' || indexerType === 'nft_prices') {
      const currentParams = paramsFields[indexerType];
      
      if (!currentParams.collection) {
        setError('Collection address is required');
        setIsLoading(false);
        return;
      }
      
      params = {
        collection: currentParams.collection,
      };
      
      // Add marketplaces if provided
      if (currentParams.marketplaces.trim()) {
        params.marketplaces = currentParams.marketplaces
          .split(',')
          .map(item => item.trim())
          .filter(Boolean);
      }
    } else if (indexerType === 'token_borrow' || indexerType === 'token_prices') {
      const currentParams = paramsFields[indexerType];
      
      if (!currentParams.tokens) {
        setError('At least one token address is required');
        setIsLoading(false);
        return;
      }
      
      params = {
        tokens: currentParams.tokens
          .split(',')
          .map(item => item.trim())
          .filter(Boolean),
      };
      
      // Add platforms if provided
      if (currentParams.platforms.trim()) {
        params.platforms = currentParams.platforms
          .split(',')
          .map(item => item.trim())
          .filter(Boolean);
      }
    }

    try {
      await IndexerService.create({
        dbCredentialId: selectedCredential,
        indexerType,
        targetTable,
        params,
      });
      
      navigate('/dashboard/indexers');
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to create indexer');
    } finally {
      setIsLoading(false);
    }
  };

  const renderParamsFields = () => {
    if (indexerType === 'nft_bids' || indexerType === 'nft_prices') {
      const currentParams = paramsFields[indexerType];
      
      return (
        <>
          <div className="space-y-2">
            <label className="text-sm font-medium" htmlFor="collection">
              Collection Address <span className="text-destructive">*</span>
            </label>
            <Input
              id="collection"
              placeholder="e.g., 8zCJ4tMmQfLbKTtPEXZhwsKQbpJMWDEp4SYxL9YA2Byn"
              value={currentParams.collection}
              onChange={(e) => handleParamsChange(indexerType, 'collection', e.target.value)}
              required
            />
            <p className="text-xs text-muted-foreground">
              Enter the Solana address of the NFT collection to index
            </p>
          </div>
          
          <div className="space-y-2">
            <label className="text-sm font-medium" htmlFor="marketplaces">
              Marketplaces (Optional)
            </label>
            <Input
              id="marketplaces"
              placeholder="e.g., MagicEden, Tensor, SolSea"
              value={currentParams.marketplaces}
              onChange={(e) => handleParamsChange(indexerType, 'marketplaces', e.target.value)}
            />
            <p className="text-xs text-muted-foreground">
              Comma-separated list of marketplaces to filter by (leave empty for all)
            </p>
          </div>
        </>
      );
    } else if (indexerType === 'token_borrow' || indexerType === 'token_prices') {
      const currentParams = paramsFields[indexerType];
      
      return (
        <>
          <div className="space-y-2">
            <label className="text-sm font-medium" htmlFor="tokens">
              Token Addresses <span className="text-destructive">*</span>
            </label>
            <Input
              id="tokens"
              placeholder="e.g., EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v,So11111111111111111111111111111111111111112"
              value={currentParams.tokens}
              onChange={(e) => handleParamsChange(indexerType, 'tokens', e.target.value)}
              required
            />
            <p className="text-xs text-muted-foreground">
              Comma-separated list of token addresses to track
            </p>
          </div>
          
          <div className="space-y-2">
            <label className="text-sm font-medium" htmlFor="platforms">
              Platforms (Optional)
            </label>
            <Input
              id="platforms"
              placeholder="e.g., Raydium, Orca, Jupiter"
              value={currentParams.platforms}
              onChange={(e) => handleParamsChange(indexerType, 'platforms', e.target.value)}
            />
            <p className="text-xs text-muted-foreground">
              Comma-separated list of platforms to filter by (leave empty for all)
            </p>
          </div>
        </>
      );
    }
    
    return null;
  };
  
  if (loadingCredentials) {
    return (
      <div className="flex justify-center items-center h-32">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }
  
  return (
    <Card className="max-w-2xl mx-auto">
      <CardHeader>
        <CardTitle>Create Blockchain Indexer</CardTitle>
        <CardDescription>
          Set up a new blockchain data indexer that will automatically sync data to your PostgreSQL database
        </CardDescription>
      </CardHeader>
      
      <form onSubmit={handleSubmit}>
        <CardContent className="space-y-6">
          {error && (
            <div className="bg-destructive/15 text-destructive text-sm p-3 rounded-md flex items-center">
              <AlertCircle className="h-4 w-4 mr-2" />
              {error}
            </div>
          )}
          
          {credentials.length === 0 ? (
            <div className="bg-primary/15 text-primary text-sm p-3 rounded-md flex items-center">
              <Info className="h-4 w-4 mr-2" />
              You need to add a database credential before creating an indexer. 
              <Button variant="link" className="p-0 h-auto ml-1" onClick={() => navigate('/dashboard/db-credentials/add')}>
                Add one now
              </Button>
            </div>
          ) : (
            <>
              <div className="space-y-2">
                <label className="text-sm font-medium" htmlFor="dbCredential">
                  Database Credential <span className="text-destructive">*</span>
                </label>
                <select
                  id="dbCredential"
                  className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                  value={selectedCredential}
                  onChange={(e) => setSelectedCredential(e.target.value)}
                  required
                >
                  {credentials.map(cred => (
                    <option key={cred.id} value={cred.id}>
                      {cred.name} ({cred.host}:{cred.port})
                    </option>
                  ))}
                </select>
              </div>
              
              <div className="space-y-2">
                <label className="text-sm font-medium" htmlFor="indexerType">
                  Indexer Type <span className="text-destructive">*</span>
                </label>
                <select
                  id="indexerType"
                  className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                  value={indexerType}
                  onChange={(e) => setIndexerType(e.target.value as IndexerType)}
                  required
                >
                  <option value="nft_bids">NFT Bids</option>
                  <option value="nft_prices">NFT Prices</option>
                  <option value="token_borrow">Token Borrowing</option>
                  <option value="token_prices">Token Prices</option>
                </select>
                <p className="text-xs text-muted-foreground">
                  Select the type of blockchain data you want to index
                </p>
              </div>
              
              <div className="space-y-2">
                <label className="text-sm font-medium" htmlFor="targetTable">
                  Target Table Name <span className="text-destructive">*</span>
                </label>
                <Input
                  id="targetTable"
                  placeholder="e.g., nft_bids_collection_name"
                  value={targetTable}
                  onChange={(e) => setTargetTable(e.target.value)}
                  required
                />
                <p className="text-xs text-muted-foreground">
                  Name of the database table where indexed data will be stored
                </p>
              </div>
              
              <div className="pt-2">
                <h3 className="text-sm font-medium mb-3">Indexer Parameters</h3>
                {renderParamsFields()}
              </div>
            </>
          )}
        </CardContent>
        
        <CardFooter className="flex justify-between">
          <Button 
            type="button" 
            variant="outline" 
            onClick={() => navigate('/dashboard/indexers')}
          >
            Cancel
          </Button>
          <Button 
            type="submit" 
            disabled={isLoading || credentials.length === 0}
          >
            {isLoading ? 'Creating...' : 'Create Indexer'}
          </Button>
        </CardFooter>
      </form>
    </Card>
  );
};

export default IndexerForm;