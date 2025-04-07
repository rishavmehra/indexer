import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { motion, AnimatePresence } from 'framer-motion';
import { 
  Card, 
  CardContent, 
  CardHeader, 
  CardTitle, 
  CardDescription,
  CardFooter 
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { ArrowLeft, ChevronRight, AlertCircle, Database, PlusCircle, Clock, Info, CheckCircle, Copy } from 'lucide-react';
import { IndexerService, DBCredentialService } from '@/services/api';
import { useToast } from '@/components/ui/toast';

interface DBCredential {
  id: string;
  host: string;
  port: number;
  name: string;
}

// Define specific interfaces for each indexer type's parameters with optional fields
interface NFTParams {
  collection?: string;
  marketplaces?: string[];
}

interface TokenParams {
  tokens?: string[];
  platforms?: string[];
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
  const [step, setStep] = useState(1);
  const [credentials, setCredentials] = useState<DBCredential[]>([]);
  const [selectedCredential, setSelectedCredential] = useState('');
  const [indexerType, setIndexerType] = useState<IndexerType>('nft_prices');
  const [targetTable, setTargetTable] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [loadingCredentials, setLoadingCredentials] = useState(true);
  const [paramsFields, setParamsFields] = useState<ParamsFieldsMap>({
    nft_bids: {
      collection: '',
      marketplaces: [],
    },
    nft_prices: {
      collection: '',
      marketplaces: [],
    },
    token_borrow: {
      tokens: [],
      platforms: [],
    },
    token_prices: {
      tokens: [],
      platforms: [],
    },
  });
  const [error, setError] = useState('');
  const [showSuccessModal, setShowSuccessModal] = useState(false);
  const [copiedExample, setCopiedExample] = useState<string | null>(null);

  const navigate = useNavigate();
  const { addToast } = useToast();

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
        addToast({
          title: 'Error',
          description: 'Failed to fetch database credentials',
          type: 'error'
        });
      } finally {
        setLoadingCredentials(false);
      }
    };

    fetchCredentials();
  }, [addToast]);

  const handleParamsChange = (
    type: IndexerType,
    field: string,
    value: string
  ) => {
    setParamsFields(prev => {
      const newParams = { ...prev };
      
      if (field === 'collection') {
        newParams[type] = {
          ...newParams[type],
          [field]: value.trim()
        };
      } else {
        newParams[type] = {
          ...newParams[type],
          [field]: value.split(',').map(item => item.trim()).filter(Boolean)
        };
      }
      
      return newParams;
    });
  };

  const validateFirstStep = () => {
    if (!selectedCredential) {
      setError('Please select a database credential');
      return false;
    }
    
    if (!targetTable) {
      setError('Please enter a target table name');
      return false;
    }
    
    setError('');
    return true;
  };

  const validateSecondStep = () => {
    const currentParams = paramsFields[indexerType];
    
    if (indexerType === 'nft_bids' || indexerType === 'nft_prices') {
      const nftParams = currentParams as NFTParams;
      if (!nftParams.collection) {
        setError('Collection address is required');
        return false;
      }
    } else if (indexerType === 'token_borrow' || indexerType === 'token_prices') {
      const tokenParams = currentParams as TokenParams;
      if (!tokenParams.tokens || tokenParams.tokens.length === 0) {
        setError('At least one token address is required');
        return false;
      }
    }
    
    setError('');
    return true;
  };

  const handleSubmit = async () => {
    if (!validateSecondStep()) return;

    setIsLoading(true);
    setError('');

    try {
      const params = paramsFields[indexerType];
      
      await IndexerService.create({
        dbCredentialId: selectedCredential,
        indexerType,
        targetTable,
        params,
      });
      
      // Show success toast
      addToast({
        title: 'Indexer Created',
        description: 'Your blockchain indexer has been created successfully.',
        type: 'success'
      });

      // Show initialization notice with longer duration
      setTimeout(() => {
        addToast({
          title: 'Important: Webhook Initialization',
          description: 'Please wait 3-4 minutes for your indexer to start working. This platform is currently in initialization state using Helius free tier.',
          type: 'info'
        });
      }, 500);
      
      // Show success modal
      setShowSuccessModal(true);
      
      // Navigate after a delay to let the user see the success modal
      setTimeout(() => {
        navigate('/dashboard/indexers');
      }, 7000);
    } catch (err: any) {
      console.error('Error creating indexer:', err);
      const errorMessage = err.response?.data?.error || 'Failed to create indexer';
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

  const navigateToCredentials = () => {
    navigate('/dashboard/db-credentials/add');
  };

  const copyToClipboard = (text: string, key: string) => {
    navigator.clipboard.writeText(text);
    setCopiedExample(key);
    setTimeout(() => setCopiedExample(null), 2000);
  };

  // Animation variants
  const modalVariants = {
    hidden: { opacity: 0, scale: 0.8 },
    visible: { 
      opacity: 1, 
      scale: 1,
      transition: { duration: 0.3 }
    },
    exit: { 
      opacity: 0, 
      scale: 0.8,
      transition: { duration: 0.2 }
    }
  };

  // Show empty state if no credentials are available
  if (!loadingCredentials && credentials.length === 0) {
    return (
      <div className="max-w-2xl mx-auto px-4">
        <motion.div 
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ duration: 0.3 }}
          className="mb-6"
        >
          <Button 
            variant="ghost" 
            onClick={() => navigate('/dashboard/indexers')}
            className="mb-4 text-muted-foreground hover:text-primary transition-colors"
          >
            <ArrowLeft className="h-4 w-4 mr-2" />
            Back to Indexers
          </Button>
          
          <h1 className="text-3xl font-bold">Create Blockchain Indexer</h1>
          <p className="text-muted-foreground mt-1">
            Set up a new blockchain data indexer that will automatically sync data to your PostgreSQL database
          </p>
        </motion.div>

        <Card className="border-border/60 shadow-lg hover:shadow-xl transition-all duration-300">
          <CardHeader className="bg-muted/30 border-b border-border/40 space-y-1">
            <CardTitle>Database Connection Required</CardTitle>
            <CardDescription>
              You need to add a database credential before creating an indexer
            </CardDescription>
          </CardHeader>
          <CardContent className="py-8 flex flex-col items-center text-center">
            <div className="w-16 h-16 rounded-full bg-primary/10 flex items-center justify-center mb-4">
              <Database className="h-8 w-8 text-primary" />
            </div>
            <h3 className="text-xl font-medium mb-2">No Database Credentials Found</h3>
            <p className="text-muted-foreground mb-8 max-w-md">
              Before creating a blockchain indexer, you need to connect at least one PostgreSQL database.
              Add your database credentials first, then come back to create your indexer.
            </p>
            <Button 
              onClick={navigateToCredentials} 
              className="flex items-center gap-2"
              size="lg"
            >
              <PlusCircle className="h-5 w-5" />
              Add Database Credential
            </Button>
          </CardContent>
          <CardFooter className="border-t border-border/40 py-4 px-6 bg-muted/30 text-sm text-muted-foreground">
            <p className="italic">
              Tip: You'll need your database host, port, name, username, and password to create a credential.
            </p>
          </CardFooter>
        </Card>
      </div>
    );
  }

  if (loadingCredentials) {
    return (
      <div className="flex justify-center items-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }

  const renderFirstStep = () => (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.3 }}
    >
      <div className="space-y-4">
        <div className="space-y-2">
          <label className="text-sm font-medium" htmlFor="dbCredential">
            Database Credential <span className="text-destructive">*</span>
          </label>
          <div className="flex gap-2">
            <select
              id="dbCredential"
              className="flex-1 rounded-md border border-input bg-background px-3 py-2 text-sm"
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
            <Button 
              variant="outline" 
              onClick={navigateToCredentials} 
              title="Add new database credential"
              className="flex-shrink-0"
            >
              <PlusCircle className="h-4 w-4" />
            </Button>
          </div>
          <p className="text-xs text-muted-foreground">
            Select the database where indexed data will be stored
          </p>
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
            <option value="nft_prices">NFT Prices</option>
            {/* <option value="token_borrow">Token Borrowing</option> */}
            <option value="token_prices">Token Prices</option>
          </select>
          <p className="text-xs text-muted-foreground">
            Select the type of blockchain data you want to index
          </p>
        </div>

        {/* Target Table Name */}
        <div className="space-y-2">
          <label className="text-sm font-medium" htmlFor="targetTable">
            Target Table Name <span className="text-destructive">*</span>
          </label>
          <Input
            id="targetTable"
            placeholder="e.g., nft_prices_collection_name"
            value={targetTable}
            onChange={(e) => setTargetTable(e.target.value)}
            required
          />
          <p className="text-xs text-muted-foreground">
            Name of the database table where indexed data will be stored
          </p>
        </div>
      </div>
    </motion.div>
  );

  const renderSecondStep = () => {
    const currentParams = paramsFields[indexerType];
    
    return (
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.3 }}
        className="space-y-4"
      >
        {(indexerType === 'nft_bids' || indexerType === 'nft_prices') && (
          <>
            <div className="space-y-2">
              <label className="text-sm font-medium" htmlFor="collection">
                Collection Address <span className="text-destructive">*</span>
              </label>
              <div className="flex flex-col gap-1">
                <div className="flex items-center gap-2 bg-muted/50 p-2 rounded-md mb-2">
                  <span className="text-xs text-muted-foreground">Add this Below(Eden Magic V2):</span>
                  <code className="text-xs bg-background px-2 py-1 rounded border">1BWutmTvYPwDtmw9abTkS4Ssr8no61spGAvW1X6NDix</code>
                  <Button 
                    variant="ghost" 
                    size="icon" 
                    className="h-6 w-6 ml-auto"
                    onClick={() => copyToClipboard("1BWutmTvYPwDtmw9abTkS4Ssr8no61spGAvW1X6NDix", "nft-collection")}
                  >
                    {copiedExample === "nft-collection" ? (
                      <CheckCircle className="h-3.5 w-3.5 text-primary" />
                    ) : (
                      <Copy className="h-3.5 w-3.5" />
                    )}
                  </Button>
                </div>
                <Input
                  id="collection"
                  placeholder="e.g., 1BWutmTvYPwDtmw9abTkS4Ssr8no61spGAvW1X6NDix"
                  value={(currentParams as NFTParams).collection || ''}
                  onChange={(e) => handleParamsChange(indexerType, 'collection', e.target.value)}
                  required
                />
              </div>
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
                value={((currentParams as NFTParams).marketplaces || []).join(', ')}
                onChange={(e) => handleParamsChange(indexerType, 'marketplaces', e.target.value)}
              />
              <p className="text-xs text-muted-foreground">
                Comma-separated list of marketplaces to filter by (leave empty for all)
              </p>
            </div>
          </>
        )}
        
        {(indexerType === 'token_borrow' || indexerType === 'token_prices') && (
          <>
            <div className="space-y-2">
              <label className="text-sm font-medium" htmlFor="tokens">
                Token Addresses <span className="text-destructive">*</span>
              </label>
              <div className="flex flex-col gap-1">
                <div className="flex items-center gap-2 bg-muted/50 p-2 rounded-md mb-2">
                  <span className="text-xs text-muted-foreground">Add this Below(Trump Token):</span>
                  <code className="text-xs bg-background px-2 py-1 rounded border">6p6xgHyF7AeE6TZkSmFsko444wqoP15icUSqi2jfGiPN</code>
                  <Button 
                    variant="ghost" 
                    size="icon" 
                    className="h-6 w-6 ml-auto"
                    onClick={() => copyToClipboard("6p6xgHyF7AeE6TZkSmFsko444wqoP15icUSqi2jfGiPN", "token-address")}
                  >
                    {copiedExample === "token-address" ? (
                      <CheckCircle className="h-3.5 w-3.5 text-primary" />
                    ) : (
                      <Copy className="h-3.5 w-3.5" />
                    )}
                  </Button>
                </div>
                <Input
                  id="tokens"
                  placeholder="e.g., 6p6xgHyF7AeE6TZkSmFsko444wqoP15icUSqi2jfGiPN"
                  value={((currentParams as TokenParams).tokens || []).join(', ')}
                  onChange={(e) => handleParamsChange(indexerType, 'tokens', e.target.value)}
                  required
                />
              </div>
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
                value={((currentParams as TokenParams).platforms || []).join(', ')}
                onChange={(e) => handleParamsChange(indexerType, 'platforms', e.target.value)}
              />
              <p className="text-xs text-muted-foreground">
                Comma-separated list of platforms to filter by (leave empty for all)
              </p>
            </div>
          </>
        )}
      </motion.div>
    );
  };

  return (
    <div className="max-w-2xl mx-auto px-4 relative">
      <motion.div 
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={{ duration: 0.3 }}
        className="mb-6"
      >
        <Button 
          variant="ghost" 
          onClick={() => navigate('/dashboard/indexers')}
          className="mb-4 text-muted-foreground hover:text-primary transition-colors"
        >
          <ArrowLeft className="h-4 w-4 mr-2" />
          Back to Indexers
        </Button>
        
        <h1 className="text-3xl font-bold">Create Blockchain Indexer</h1>
        <p className="text-muted-foreground mt-1">
          Set up a new blockchain data indexer that will automatically sync data to your PostgreSQL database
        </p>
      </motion.div>

      {/* Progress steps */}
      <motion.div 
        className="mb-8 relative"
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={{ delay: 0.2 }}
      >
        <div className="flex justify-between">
          <div className={`flex flex-col items-center ${step >= 1 ? 'text-primary' : 'text-muted-foreground'}`}>
            <div className={`h-10 w-10 rounded-full flex items-center justify-center mb-2 ${step >= 1 ? 'bg-primary text-white' : 'bg-muted'}`}>
              1
            </div>
            <span className="text-sm font-medium">Indexer Details</span>
          </div>
          
          <div className={`flex flex-col items-center ${step >= 2 ? 'text-primary' : 'text-muted-foreground'}`}>
            <div className={`h-10 w-10 rounded-full flex items-center justify-center mb-2 ${step >= 2 ? 'bg-primary text-white' : 'bg-muted'}`}>
              2
            </div>
            <span className="text-sm font-medium">Indexer Parameters</span>
          </div>
        </div>
        
        {/* Progress line */}
        <div className="absolute top-5 left-0 right-0 mx-auto w-3/4 h-[2px] bg-muted -z-10" />
        <motion.div 
          className="absolute top-5 left-0 mx-auto h-[2px] bg-primary -z-10"
          initial={{ width: "0%" }}
          animate={{ width: step === 1 ? "37.5%" : "100%" }}
          transition={{ duration: 0.5 }}
        />
      </motion.div>

      <Card className="border-border/60 shadow-lg hover:shadow-xl transition-all duration-300">
        <CardHeader className="bg-muted/30 border-b border-border/40 space-y-1">
          <CardTitle>{step === 1 ? 'Indexer Details' : 'Indexer Parameters'}</CardTitle>
          <CardDescription>
            {step === 1 
              ? 'Configure basic details for your blockchain indexer' 
              : 'Specify the parameters for data indexing'}
          </CardDescription>
        </CardHeader>
        
        <CardContent className="space-y-6 py-6">
          {error && (
            <div className="bg-destructive/15 text-destructive text-sm p-3 rounded-md flex items-center">
              <AlertCircle className="h-4 w-4 mr-2 flex-shrink-0" />
              {error}
            </div>
          )}
          {step === 1 ? renderFirstStep() : renderSecondStep()}
        </CardContent>
        
        <div className="border-t border-border/40 py-5 px-6 bg-muted/30 flex justify-between">
          {step === 1 ? (
            <Button 
              type="button" 
              variant="outline" 
              onClick={() => navigate('/dashboard/indexers')}
              className="relative overflow-hidden group"
            >
              <span className="relative z-10">Cancel</span>
              <span className="absolute inset-0 bg-background hover:bg-muted transition-colors duration-200"></span>
            </Button>
          ) : (
            <Button 
              type="button" 
              variant="outline" 
              onClick={() => setStep(1)}
              className="relative overflow-hidden group"
            >
              <ArrowLeft className="mr-2 h-4 w-4 relative z-10" />
              <span className="relative z-10">Back</span>
              <span className="absolute inset-0 bg-background hover:bg-muted transition-colors duration-200"></span>
            </Button>
          )}
          
          {step === 1 ? (
            <Button 
              type="button" 
              disabled={isLoading}
              onClick={() => {
                if (validateFirstStep()) {
                  setStep(2);
                }
              }}
              className="relative overflow-hidden group"
            >
              <span className="relative z-10 flex items-center">
                Continue
                <ChevronRight className="ml-2 h-4 w-4" />
              </span>
              <span className="absolute inset-0 bg-gradient-to-r from-primary/80 to-primary opacity-0 group-hover:opacity-100 transition-opacity duration-300"></span>
            </Button>
          ) : (
            <Button 
              type="button" 
              disabled={isLoading}
              onClick={handleSubmit}
              className="relative overflow-hidden group"
            >
              <span className="relative z-10">
                {isLoading ? (
                  <span className="flex items-center">
                    <svg className="animate-spin -ml-1 mr-2 h-4 w-4 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                    </svg>
                    Creating Indexer...
                  </span>
                ) : 'Create Indexer'}
              </span>
              <span className="absolute inset-0 bg-gradient-to-r from-primary/80 to-primary opacity-0 group-hover:opacity-100 transition-opacity duration-300"></span>
            </Button>
          )}
        </div>
      </Card>

      {/* Success Modal */}
      <AnimatePresence>
        {showSuccessModal && (
          <motion.div
            className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50"
            initial="hidden"
            animate="visible"
            exit="hidden"
            variants={{
              hidden: { opacity: 0 },
              visible: { opacity: 1 }
            }}
          >
            <motion.div
              className="bg-card border border-border rounded-lg shadow-xl max-w-md w-full p-6"
              variants={modalVariants}
            >
              <div className="flex flex-col items-center text-center">
                <div className="w-16 h-16 rounded-full bg-green-100 dark:bg-green-900/30 flex items-center justify-center mb-4">
                  <CheckCircle className="h-8 w-8 text-green-600 dark:text-green-400" />
                </div>
                <h3 className="text-xl font-bold mb-2">Indexer Created Successfully</h3>
                <p className="text-muted-foreground mb-4">
                  Your indexer has been created and is being initialized.
                </p>
                
                <div className="bg-yellow-100 dark:bg-yellow-900/30 p-4 rounded-md border border-yellow-200 dark:border-yellow-800 mb-4 flex">
                  <Info className="h-5 w-5 text-yellow-600 dark:text-yellow-400 mr-3 flex-shrink-0 mt-0.5" />
                  <div>
                    <p className="text-yellow-800 dark:text-yellow-300 font-medium">Please wait 3-4 minutes</p>
                    <p className="text-yellow-700 dark:text-yellow-400 text-sm mt-1">
                      This platform is currently in initialization state using Helius free tier. 
                      The webhook needs a few minutes to start working properly.
                    </p>
                  </div>
                </div>
                
                <div className="flex items-center justify-center gap-2 mt-2">
                  <Clock className="h-4 w-4 text-muted-foreground animate-pulse" />
                  <span className="text-sm text-muted-foreground">
                    Redirecting to indexers page...
                  </span>
                </div>
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
};

export default IndexerForm;
