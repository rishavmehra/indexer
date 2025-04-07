import React, { useState, useEffect, useCallback, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { AlertCircle, RefreshCw, ExternalLink, Copy, Clock, Search, Filter, Check, PlayCircle, PauseCircle, RotateCw, Coins, ArrowDownUp } from 'lucide-react';
import { IndexerService } from '@/services/api';
import { motion } from 'framer-motion';
import { useToast } from '@/components/ui/toast';

interface IndexerLog {
  id: string;
  indexerId: string;
  eventType: string;
  message: string;
  details: any;
  createdAt: string;
}

// NFT Transaction Interface
interface NftTransaction {
  block_time: string;
  buyer: string | null;
  created_at: string;
  currency: string;
  id: string;
  marketplace: string;
  nft_mint: string;
  nft_name: string;
  price: number;
  seller: string;
  signature: string;
  slot: number;
  status: string;
  updated_at: string;
  usd_value: number | null;
}

// Token Transaction Interface
interface TokenTransaction {
  platform: string;
  price_sol: number;
  price_usd: number | null;
  slot: number;
  token_address: string;
  token_name: string;
  token_symbol: string;
  transaction_id: string;
  updated_at: string;
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
  const [nftTransactions, setNftTransactions] = useState<NftTransaction[]>([]);
  const [tokenTransactions, setTokenTransactions] = useState<TokenTransaction[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  const [limit] = useState(100);
  const [offset] = useState(0);
  const [searchTerm, setSearchTerm] = useState('');
  const [selectedStatus, setSelectedStatus] = useState<string>('All Statuses');
  const [selectedPlatform, setSelectedPlatform] = useState<string>('All Platforms');
  const [activeTab, setActiveTab] = useState<'transactions' | 'system'>('transactions');
  const [copiedSignature, setCopiedSignature] = useState<string | null>(null);
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [refreshInterval, setRefreshInterval] = useState(5000); // 5 seconds default
  const [lastRefreshTime, setLastRefreshTime] = useState<Date | null>(null);
  const autoRefreshIntervalRef = useRef<number | null>(null);
  const navigate = useNavigate();
  const { addToast } = useToast();

  // Extract NFT transactions from logs
  const extractNftTransactions = useCallback((logs: IndexerLog[]) => {
    const allTransactions: NftTransaction[] = [];

    logs.forEach(log => {
      if (log.details?.transactions && Array.isArray(log.details.transactions)) {
        log.details.transactions.forEach((tx: NftTransaction) => {
          allTransactions.push({
            ...tx,
            // Ensure all required fields are present
            block_time: tx.block_time || log.createdAt,
            created_at: tx.created_at || log.createdAt,
            updated_at: tx.updated_at || log.createdAt
          });
        });
      }
    });

    // Sort by time, newest first
    return allTransactions.sort((a, b) =>
      new Date(b.block_time).getTime() - new Date(a.block_time).getTime()
    );
  }, []);

  // Extract Token transactions from logs
  const extractTokenTransactions = useCallback((logs: IndexerLog[]) => {
    const allTransactions: TokenTransaction[] = [];

    logs.forEach(log => {
      if (log.details?.tokens && Array.isArray(log.details.tokens)) {
        log.details.tokens.forEach((tx: TokenTransaction) => {
          allTransactions.push({
            ...tx,
            // Ensure all required fields are present
            updated_at: tx.updated_at || log.createdAt
          });
        });
      }
    });

    // Sort by time, newest first
    return allTransactions.sort((a, b) =>
      new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
    );
  }, []);

  const fetchIndexerAndLogs = useCallback(async () => {
    if (!id) return;

    try {
      setIsLoading(true);

      // Get indexer details
      const indexerResponse = await IndexerService.getById(id);
      setIndexer(indexerResponse.data);

      // Get indexer logs
      const logsResponse = await IndexerService.getLogs(id, limit, offset);
      setLogs(logsResponse.data);

      // Extract transactions based on indexer type
      if (indexerResponse.data.indexerType.includes('nft')) {
        // For NFT indexers
        const nftTxs = extractNftTransactions(logsResponse.data);
        setNftTransactions(nftTxs);
        setTokenTransactions([]);
      } else if (indexerResponse.data.indexerType.includes('token')) {
        // For Token indexers
        const tokenTxs = extractTokenTransactions(logsResponse.data);
        setTokenTransactions(tokenTxs);
        setNftTransactions([]);
      }

      setError('');
      setLastRefreshTime(new Date());
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to fetch indexer logs');
    } finally {
      setIsLoading(false);
    }
  }, [id, limit, offset, extractNftTransactions, extractTokenTransactions]);

  // Setup auto-refresh
  useEffect(() => {
    if (autoRefresh) {
      // Clear any existing interval first
      if (autoRefreshIntervalRef.current) {
        clearInterval(autoRefreshIntervalRef.current);
      }
      
      // Set up new interval
      autoRefreshIntervalRef.current = window.setInterval(() => {
        fetchIndexerAndLogs();
      }, refreshInterval);
      
      // Inform user that auto-refresh is enabled
      addToast({
        title: 'Auto-refresh Enabled',
        description: `Refreshing logs every ${refreshInterval / 1000} seconds`,
        type: 'info'
      });
    } else {
      // Clear interval when auto-refresh is disabled
      if (autoRefreshIntervalRef.current) {
        clearInterval(autoRefreshIntervalRef.current);
        autoRefreshIntervalRef.current = null;
      }
    }

    // Clean up on unmount
    return () => {
      if (autoRefreshIntervalRef.current) {
        clearInterval(autoRefreshIntervalRef.current);
      }
    };
  }, [autoRefresh, refreshInterval, fetchIndexerAndLogs, addToast]);

  // Initial fetch
  useEffect(() => {
    fetchIndexerAndLogs();
  }, [fetchIndexerAndLogs]);

  const refreshLogs = () => {
    fetchIndexerAndLogs();
  };

  const toggleAutoRefresh = () => {
    setAutoRefresh(!autoRefresh);
  };

  const formatDateTime = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleString();
  };

  const formatPrice = (price: number, currency: string, usdValue: number | null) => {
    if (currency === 'SOL') {
      return (
        <div>
          <div>{price.toFixed(4)} SOL</div>
          {usdValue && <div className="text-xs text-muted-foreground">${usdValue.toFixed(2)}</div>}
        </div>
      );
    }
    return `${price.toFixed(2)} ${currency}`;
  };

  const getMarketplaceLink = (marketplace: string, mintAddress: string) => {
    switch (marketplace.toUpperCase()) {
      case 'MAGIC_EDEN':
        return `https://magiceden.io/item-details/${mintAddress}`;
      case 'TENSOR':
        return `https://tensor.trade/item/${mintAddress}`;
      case 'SOLSEA':
        return `https://solsea.io/nft/${mintAddress}`;
      default:
        return `https://explorer.solana.com/address/${mintAddress}`;
    }
  };

  const getPlatformLink = (platform: string, tokenAddress: string) => {
    switch (platform.toUpperCase()) {
      case 'JUPITER':
        return `https://jup.ag/swap/${tokenAddress}-SOL`;
      case 'ORCA':
        return `https://www.orca.so/`;
      case 'RAYDIUM':
        return `https://raydium.io/swap/?inputCurrency=${tokenAddress}&outputCurrency=SOL`;
      default:
        return `https://explorer.solana.com/address/${tokenAddress}`;
    }
  };

  const getStatusClass = (status: string) => {
    switch (status.toLowerCase()) {
      case 'listed':
        return 'text-xs font-medium rounded-full px-2.5 py-0.5 bg-blue-900/30 text-blue-400';
      case 'sold':
        return 'text-xs font-medium rounded-full px-2.5 py-0.5 bg-green-900/30 text-green-400';
      case 'cancelled':
        return 'text-xs font-medium rounded-full px-2.5 py-0.5 bg-yellow-900/30 text-yellow-400';
      default:
        return 'text-xs font-medium rounded-full px-2.5 py-0.5 bg-gray-900/30 text-gray-400';
    }
  };

  const getSolanaFmLink = (signature: string) => {
    return `https://solana.fm/tx/${signature}`;
  };

  const getExplorerLink = (address: string) => {
    return `https://explorer.solana.com/address/${address}`;
  };

  const truncateAddress = (address: string | null) => {
    if (!address) return 'N/A';
    return `${address.slice(0, 6)}...${address.slice(-4)}`;
  };

  const copyToClipboard = (text: string, signatureId: string) => {
    navigator.clipboard.writeText(text);
    setCopiedSignature(signatureId);
    setTimeout(() => setCopiedSignature(null), 2000);
  };

  const filterNftTransactions = () => {
    return nftTransactions.filter(tx => {
      // Filter by search term
      const matchesSearch = searchTerm === '' ||
        tx.nft_name.toLowerCase().includes(searchTerm.toLowerCase()) ||
        tx.nft_mint.toLowerCase().includes(searchTerm.toLowerCase()) ||
        tx.signature.toLowerCase().includes(searchTerm.toLowerCase()) ||
        (tx.seller && tx.seller.toLowerCase().includes(searchTerm.toLowerCase())) ||
        (tx.buyer && tx.buyer.toLowerCase().includes(searchTerm.toLowerCase()));

      // Filter by status
      const matchesStatus = selectedStatus === 'All Statuses' ||
        tx.status.toLowerCase() === selectedStatus.toLowerCase();

      return matchesSearch && matchesStatus;
    });
  };

  const filterTokenTransactions = () => {
    return tokenTransactions.filter(tx => {
      // Filter by search term
      const matchesSearch = searchTerm === '' ||
        tx.token_name.toLowerCase().includes(searchTerm.toLowerCase()) ||
        tx.token_symbol.toLowerCase().includes(searchTerm.toLowerCase()) ||
        tx.token_address.toLowerCase().includes(searchTerm.toLowerCase()) ||
        tx.transaction_id.toLowerCase().includes(searchTerm.toLowerCase());

      // Filter by platform
      const matchesPlatform = selectedPlatform === 'All Platforms' ||
        tx.platform.toLowerCase() === selectedPlatform.toLowerCase();

      return matchesSearch && matchesPlatform;
    });
  };

  const renderSystemLogs = () => {
    if (logs.length === 0) {
      return (
        <div className="text-center p-8 text-muted-foreground">
          No system logs found
        </div>
      );
    }

    return (
      <div className="space-y-4">
        {logs.map(log => (
          <div key={log.id} className="border border-border/60 rounded-md p-4 bg-card/30">
            <div className="flex justify-between mb-2">
              <div className="font-medium">{log.eventType}</div>
              <div className="text-sm text-muted-foreground">{formatDateTime(log.createdAt)}</div>
            </div>
            <div className="text-sm mb-3">{log.message}</div>
            {log.details && Object.keys(log.details).length > 0 && (
              <div className="bg-muted/50 rounded p-3 text-xs overflow-x-auto">
                <pre>{JSON.stringify(log.details, null, 2)}</pre>
              </div>
            )}
          </div>
        ))}
      </div>
    );
  };

  const renderNftTransactions = () => {
    const filteredTransactions = filterNftTransactions();
    
    if (filteredTransactions.length === 0) {
      return (
        <div className="text-center p-12 text-muted-foreground">
          No NFT transactions found
        </div>
      );
    }

    return (
      <div className="overflow-x-auto">
        <table className="w-full">
          <thead>
            <tr className="border-b border-border/60 text-left text-muted-foreground">
              <th className="p-4 font-medium flex items-center">
                <Clock className="h-4 w-4 mr-2" />
                Time
              </th>
              <th className="p-4 font-medium">NFT</th>
              <th className="p-4 font-medium">Price</th>
              <th className="p-4 font-medium">Seller/Buyer</th>
              <th className="p-4 font-medium">Market</th>
              <th className="p-4 font-medium">Status</th>
              <th className="p-4 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {filteredTransactions.map((tx) => (
              <tr key={tx.signature} className="border-b border-border/60 hover:bg-muted/20">
                <td className="p-4 whitespace-nowrap">
                  {formatDateTime(tx.block_time)}
                </td>
                <td className="p-4">
                  <div className="font-medium">{tx.nft_name}</div>
                  <div className="text-sm text-muted-foreground">
                    {truncateAddress(tx.nft_mint)}
                  </div>
                </td>
                <td className="p-4 whitespace-nowrap">
                  {formatPrice(tx.price, tx.currency, tx.usd_value)}
                </td>
                <td className="p-4">
                  <div className="flex flex-col gap-1">
                    <div className="flex items-center gap-1">
                      <span className="text-xs text-muted-foreground">S:</span>
                      <a
                        href={getExplorerLink(tx.seller)}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-sm text-primary hover:underline"
                      >
                        {truncateAddress(tx.seller)}
                      </a>
                    </div>
                    {tx.buyer && (
                      <div className="flex items-center gap-1">
                        <span className="text-xs text-muted-foreground">B:</span>
                        <a
                          href={getExplorerLink(tx.buyer)}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="text-sm text-primary hover:underline"
                        >
                          {truncateAddress(tx.buyer)}
                        </a>
                      </div>
                    )}
                  </div>
                </td>
                <td className="p-4">
                  <a
                    href={getMarketplaceLink(tx.marketplace, tx.nft_mint)}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-primary flex items-center hover:underline"
                  >
                    {tx.marketplace.replace('_', ' ')}
                    <ExternalLink className="h-3 w-3 ml-1" />
                  </a>
                </td>
                <td className="p-4">
                  <span className={getStatusClass(tx.status)}>
                    {tx.status.toUpperCase()}
                  </span>
                </td>
                <td className="p-4">
                  <div className="flex items-center gap-2">
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8"
                      onClick={() => copyToClipboard(tx.signature, tx.signature)}
                      title="Copy signature"
                    >
                      {copiedSignature === tx.signature ? (
                        <Check className="h-4 w-4 text-green-500" />
                      ) : (
                        <Copy className="h-4 w-4" />
                      )}
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8"
                      onClick={() => window.open(getSolanaFmLink(tx.signature), '_blank')}
                      title="View on Solana.fm"
                    >
                      <ExternalLink className="h-4 w-4" />
                    </Button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    );
  };

  const renderTokenTransactions = () => {
    const filteredTransactions = filterTokenTransactions();
    
    if (filteredTransactions.length === 0) {
      return (
        <div className="text-center p-12 text-muted-foreground">
          No token transactions found
        </div>
      );
    }

    return (
      <div className="overflow-x-auto">
        <table className="w-full">
          <thead>
            <tr className="border-b border-border/60 text-left text-muted-foreground">
              <th className="p-4 font-medium flex items-center">
                <Clock className="h-4 w-4 mr-2" />
                Time
              </th>
              <th className="p-4 font-medium">Token</th>
              <th className="p-4 font-medium">Swap Amount</th>
              <th className="p-4 font-medium">Platform</th>
              <th className="p-4 font-medium">Slot</th>
              <th className="p-4 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {filteredTransactions.map((tx) => (
              <tr key={`${tx.transaction_id}-${tx.platform}`} className="border-b border-border/60 hover:bg-muted/20">
                <td className="p-4 whitespace-nowrap">
                  {formatDateTime(tx.updated_at)}
                </td>
                <td className="p-4">
                  <div className="font-medium">{tx.token_name}</div>
                  <div className="text-sm text-muted-foreground flex items-center gap-1">
                    <span>{tx.token_symbol}</span>
                    <span className="text-xs text-muted-foreground">({truncateAddress(tx.token_address)})</span>
                  </div>
                </td>
                <td className="p-4 whitespace-nowrap">
                  <div className="flex flex-col">
                    <div className="font-medium">{tx.price_sol.toFixed(6)} SOL</div>
                    {tx.price_usd && tx.price_usd > 0 && (
                      <div className="text-xs text-muted-foreground">${tx.price_usd.toFixed(2)} USD</div>
                    )}
                  </div>
                </td>
                <td className="p-4">
                  <a
                    href={getPlatformLink(tx.platform, tx.token_address)}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-primary flex items-center hover:underline"
                  >
                    {tx.platform.replace('_', ' ')}
                    <ExternalLink className="h-3 w-3 ml-1" />
                  </a>
                </td>
                <td className="p-4">
                  {tx.slot > 0 ? tx.slot.toLocaleString() : 'N/A'}
                </td>
                <td className="p-4">
                  <div className="flex items-center gap-2">
                    {tx.transaction_id && (
                      <>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8"
                          onClick={() => copyToClipboard(tx.transaction_id, tx.transaction_id)}
                          title="Copy transaction ID"
                        >
                          {copiedSignature === tx.transaction_id ? (
                            <Check className="h-4 w-4 text-green-500" />
                          ) : (
                            <Copy className="h-4 w-4" />
                          )}
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8"
                          onClick={() => window.open(getSolanaFmLink(tx.transaction_id), '_blank')}
                          title="View on Solana.fm"
                        >
                          <ExternalLink className="h-4 w-4" />
                        </Button>
                      </>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    );
  };

  if (isLoading && !lastRefreshTime) {
    return (
      <div className="flex justify-center items-center h-64">
        <motion.div
          className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"
          animate={{ rotate: 360 }}
          transition={{ duration: 1, repeat: Infinity, ease: "linear" }}
        />
      </div>
    );
  }

  // Get unique platforms for token filtering
  const platforms = ['All Platforms', ...new Set(tokenTransactions.map(tx => tx.platform))];

  // Determine if we're showing token or NFT data
  const isTokenIndexer = indexer?.indexerType.includes('token');

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => navigate('/dashboard/indexers')}
            className="gap-1"
          >
            <span className="text-muted-foreground">←</span>
            <span>Back to Indexers</span>
          </Button>
          <h1 className="text-xl font-bold">
            {indexerType(indexer?.indexerType)} Logs: {indexer?.targetTable}
          </h1>
        </div>
        <div className="flex items-center gap-2">
          {isLoading && (
            <div className="text-xs text-muted-foreground flex items-center mr-2">
              <RotateCw className="h-3 w-3 animate-spin mr-1" />
              Refreshing...
            </div>
          )}
          
          <div className="flex items-center gap-1 text-sm text-muted-foreground">
            <Clock className="h-4 w-4" />
            {lastRefreshTime ? (
              <span title={lastRefreshTime.toLocaleString()}>
                Last refreshed: {formatRelativeTime(lastRefreshTime)}
              </span>
            ) : (
              <span>Never refreshed</span>
            )}
          </div>
          
          <div className="flex border rounded-md overflow-hidden">
            <Button
              variant={autoRefresh ? "default" : "outline"}
              size="sm"
              onClick={toggleAutoRefresh}
              className="rounded-r-none border-r"
            >
              {autoRefresh ? (
                <div className="flex items-center gap-1">
                  <PauseCircle className="h-4 w-4" />
                  <span>Auto</span>
                </div>
              ) : (
                <div className="flex items-center gap-1">
                  <PlayCircle className="h-4 w-4" />
                  <span>Auto</span>
                </div>
              )}
            </Button>
            <select
              className="px-2 py-1 text-xs border-none focus:ring-0 bg-background"
              value={refreshInterval}
              onChange={(e) => setRefreshInterval(Number(e.target.value))}
              title="Refresh interval"
            >
              <option value="3000">3s</option>
              <option value="5000">5s</option>
              <option value="10000">10s</option>
              <option value="30000">30s</option>
              <option value="60000">1m</option>
            </select>
          </div>
          
          <Button
            variant="outline"
            size="sm"
            onClick={refreshLogs}
            className="flex items-center gap-2"
            disabled={isLoading}
          >
            <RefreshCw className={`h-4 w-4 ${isLoading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
        </div>
      </div>

      {/* Tabs */}
      <div className="mb-4 border-b border-border">
        <div className="flex gap-4">
          <Button
            variant={activeTab === 'transactions' ? 'default' : 'ghost'}
            onClick={() => setActiveTab('transactions')}
            className={`rounded-none border-b-2 px-4 py-2 ${activeTab === 'transactions'
                ? 'border-primary text-white'
                : 'border-transparent text-muted-foreground'
              }`}
            size="sm"
          >
            {isTokenIndexer ? (
              <>
                <Coins className="h-4 w-4 mr-2" />
                Token Transactions
              </>
            ) : (
              <>
                <ArrowDownUp className="h-4 w-4 mr-2" />
                NFT Transactions
              </>
            )}
          </Button>
          <Button
            variant={activeTab === 'system' ? 'default' : 'ghost'}
            onClick={() => setActiveTab('system')}
            className={`rounded-none border-b-2 px-4 py-2 ${activeTab === 'system'
                ? 'border-primary text-white '
                : 'border-transparent text-muted-foreground'
              }`}
            size="sm"
          >
            <Clock className="h-4 w-4 mr-2" />
            System Logs
          </Button>
        </div>
      </div>

      {/* Error Message */}
      {error && (
        <div className="bg-destructive/15 text-destructive text-sm p-3 rounded-md flex items-center mb-4">
          <AlertCircle className="h-4 w-4 mr-2" />
          {error}
        </div>
      )}

      {/* Filter Controls */}
      <div className="flex flex-col sm:flex-row gap-3 mb-4">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder={isTokenIndexer 
              ? "Search by token name, symbol, address, or transaction ID..." 
              : "Search by NFT name, address, or signature..."}
            className="pl-9 bg-background"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
          />
        </div>
        <div className="flex gap-2">
          {isTokenIndexer ? (
            <select
              className="px-3 py-2 rounded-md border border-input bg-background h-10 text-sm min-w-[150px]"
              value={selectedPlatform}
              onChange={(e) => setSelectedPlatform(e.target.value)}
            >
              {platforms.map(platform => (
                <option key={platform} value={platform}>{platform.replace('_', ' ')}</option>
              ))}
            </select>
          ) : (
            <select
              className="px-3 py-2 rounded-md border border-input bg-background h-10 text-sm min-w-[150px]"
              value={selectedStatus}
              onChange={(e) => setSelectedStatus(e.target.value)}
            >
              <option value="All Statuses">All Statuses</option>
              <option value="listed">Listed</option>
              <option value="sold">Sold</option>
              <option value="cancelled">Cancelled</option>
            </select>
          )}
          <Button variant="outline" size="icon" className="h-10 w-10">
            <Filter className="h-4 w-4" />
          </Button>
        </div>
      </div>

      {/* Content based on active tab */}
      {activeTab === 'transactions' ? (
        <div className="bg-card/30 rounded-lg border border-border/60 overflow-hidden">
          <div className="p-4 bg-muted/30 border-b border-border/60">
            <h2 className="text-lg font-medium">
              {isTokenIndexer ? 'Token Transactions' : 'NFT Transactions'}
            </h2>
            <p className="text-sm text-muted-foreground">
              Data from the {indexer?.targetTable} table in your database
            </p>
          </div>

          {isTokenIndexer ? renderTokenTransactions() : renderNftTransactions()}
        </div>
      ) : (
        <div className="bg-card/30 rounded-lg border border-border/60 p-4 h-full overflow-y-auto">
          {renderSystemLogs()}
        </div>
      )}
    </div>
  );
};

// Helper function to format indexer type
const indexerType = (type: string | undefined): string => {
  if (!type) return 'Indexer';
  return type.replace(/_/g, ' ').toUpperCase();
};

// Helper function to format relative time
const formatRelativeTime = (date: Date): string => {
  const now = new Date();
  const seconds = Math.floor((now.getTime() - date.getTime()) / 1000);
  
  if (seconds < 10) {
    return 'just now';
  } else if (seconds < 60) {
    return `${seconds} seconds ago`;
  } else if (seconds < 3600) {
    const minutes = Math.floor(seconds / 60);
    return `${minutes} minute${minutes === 1 ? '' : 's'} ago`;
  } else if (seconds < 86400) {
    const hours = Math.floor(seconds / 3600);
    return `${hours} hour${hours === 1 ? '' : 's'} ago`;
  } else {
    const days = Math.floor(seconds / 86400);
    return `${days} day${days === 1 ? '' : 's'} ago`;
  }
};

export default IndexerLogs;
