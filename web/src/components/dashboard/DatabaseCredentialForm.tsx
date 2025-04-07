import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card, CardContent, CardFooter, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { AlertCircle, Database, Server, Globe, Key, Lock, Tag, ArrowLeft, ChevronRight, Info, ShieldCheck, Copy, Check, LinkIcon, BarChart3, CheckCircle } from 'lucide-react';
import { DBCredentialService } from '@/services/api';
import { motion } from 'framer-motion';
import { PasswordInput } from '@/components/PasswordInput';
import { useToast } from '@/components/ui/toast';

const DatabaseCredentialForm = ({ credentialId, isEditing = false }: { credentialId: string, isEditing: boolean }) => {
  const [formData, setFormData] = useState({
    host: '',
    port: 5432,
    name: '',
    user: '',
    password: '',
    sslMode: 'disable'
  });

  const [step, setStep] = useState(1);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');
  const [isCopied, setIsCopied] = useState(false);
  const [connectionUrl, setConnectionUrl] = useState('');
  const [credentialSaved, setCredentialSaved] = useState(false);
  const navigate = useNavigate();
  const { addToast } = useToast();

  useEffect(() => {
    if (isEditing && credentialId) {
      const fetchCredential = async () => {
        try {
          setIsLoading(true);
          const response = await DBCredentialService.getById(credentialId);
          const credential = response.data;

          setFormData({
            host: credential.host,
            port: credential.port,
            name: credential.name,
            user: credential.user,
            password: '',
            sslMode: credential.sslMode || 'disable'
          });
        } catch (err) {
          setError((err as any).response?.data?.error || 'Failed to fetch database credential');
        } finally {
          setIsLoading(false);
        }
      };

      fetchCredential();
    }
  }, [credentialId, isEditing]);

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    setFormData(prev => ({
      ...prev,
      [name]: name === 'port' ? parseInt(value, 10) || '' : value
    }));
  };

  const parseConnectionUrl = () => {
    if (!connectionUrl) {
      setError('Please enter a connection URL');
      return;
    }

    try {
      const regex = /^postgresql:\/\/(?:([^:@]+)(?::([^@]+))?@)?([^:/?]+)(?::(\d+))?\/([^?]+)(?:\?(.+))?$/;
      const match = connectionUrl.match(regex);

      if (!match) {
        setError('Invalid PostgreSQL connection URL format');
        return;
      }

      const [, user, password, host, port, dbname, queryParams] = match;

      let sslMode = 'disable';
      if (queryParams) {
        const params = new URLSearchParams(queryParams);
        const sslmodeParam = params.get('sslmode');
        if (sslmodeParam) {
          const allowedSslModes = ['disable', 'require', 'verify-ca', 'verify-full'];
          if (allowedSslModes.includes(sslmodeParam)) {
            sslMode = sslmodeParam;
          }
        }
      }

      setFormData({
        host: host || '',
        port: port ? parseInt(port, 10) : 5432,
        name: dbname || '',
        user: user || '',
        password: password || '',
        sslMode: sslMode
      });

      addToast({
        title: 'Connection URL Parsed',
        description: 'Database details have been extracted from the URL',
        type: 'success'
      });

      setError('');
    } catch (err) {
      setError('Failed to parse connection URL. Please check the format.');
    }
  };

  const testConnection = async () => {
    setIsLoading(true);
    setError('');

    if (connectionUrl && !formData.host) {
      parseConnectionUrl();
      if (error) {
        setIsLoading(false);
        return;
      }
    }

    if (!formData.host || !formData.port || !formData.name || !formData.user || (!isEditing && !formData.password)) {
      setError('Please fill in all required fields or provide a valid connection URL');
      setIsLoading(false);
      return;
    }

    try {
      const response = await DBCredentialService.testConnection({
        host: formData.host,
        port: formData.port,
        name: formData.name,
        user: formData.user,
        password: formData.password,
        sslMode: formData.sslMode
      });

      if (response.data.success) {
        addToast({
          title: 'Connection Successful',
          description: 'Successfully connected to your PostgreSQL database.',
          type: 'success'
        });
        setStep(2);
      } else {
        setError(response.data.error || 'Connection failed. Please check your credentials.');
      }
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to connect to database. Please check your credentials.');
    } finally {
      setIsLoading(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setIsLoading(true);
    setError('');

    try {
      if (isEditing && credentialId) {
        await DBCredentialService.update(credentialId, formData);
        addToast({
          title: 'Credential Updated',
          description: 'Database credential has been updated successfully.',
          type: 'success'
        });
        navigate('/dashboard/db-credentials');
      } else {
        await DBCredentialService.create(formData);
        addToast({
          title: 'Credential Created',
          description: 'Database credential has been created successfully.',
          type: 'success'
        });
        setCredentialSaved(true);
      }
    } catch (err) {
      setError((err as any).response?.data?.error || 'Failed to save database credential');
    } finally {
      setIsLoading(false);
    }
  };

  const copyConnectionString = () => {
    const connectionString = `postgresql://${formData.user}:******@${formData.host}:${formData.port}/${formData.name}`;
    navigator.clipboard.writeText(connectionString.replace('******', formData.password));
    setIsCopied(true);
    setTimeout(() => setIsCopied(false), 2000);
  };

  const cardVariants = {
    hidden: { opacity: 0, y: 20 },
    visible: {
      opacity: 1,
      y: 0,
      transition: { duration: 0.4 }
    },
    exit: {
      opacity: 0,
      y: -20,
      transition: { duration: 0.3 }
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
    hidden: { opacity: 0, y: 10 },
    visible: { opacity: 1, y: 0 }
  };

  if (credentialSaved) {
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
            onClick={() => navigate('/dashboard/db-credentials')}
            className="mb-4 text-muted-foreground hover:text-primary transition-colors"
          >
            <ArrowLeft className="h-4 w-4 mr-2" />
            Back to credentials
          </Button>

          <h1 className="text-3xl font-bold">Database Connected Successfully</h1>
          <p className="text-muted-foreground mt-1">
            Your database credentials have been added successfully.
          </p>
        </motion.div>

        <Card className="border-border/60 shadow-lg hover:shadow-xl transition-all duration-300">
          <CardHeader className="bg-green-100 dark:bg-green-900/30 border-b border-border/40 space-y-1">
            <CardTitle className="flex items-center gap-2 text-green-800 dark:text-green-400">
              <CheckCircle className="h-5 w-5" />
              Connection Successful
            </CardTitle>
            <CardDescription className="text-green-700 dark:text-green-500">
              Your database credential has been saved and is ready to use
            </CardDescription>
          </CardHeader>

          <CardContent className="py-6 space-y-6">
            <div className="flex flex-col items-center text-center py-4">
              <div className="w-16 h-16 rounded-full bg-primary/10 flex items-center justify-center mb-4">
                <Database className="h-8 w-8 text-primary" />
              </div>
              <h3 className="text-xl font-medium mb-2">Ready to Create an Indexer</h3>
              <p className="text-muted-foreground mb-8 max-w-md">
                Now that your database is connected, you can create blockchain indexers
                to automatically sync on-chain data to your PostgreSQL database.
              </p>
            </div>

            <div className="bg-muted/30 rounded-lg p-4 flex items-start gap-4">
              <div className="flex-shrink-0 mt-1">
                <Info className="h-5 w-5 text-primary" />
              </div>
              <div className="space-y-2">
                <h4 className="font-medium">What's Next?</h4>
                <p className="text-sm text-muted-foreground">
                  You can now create a blockchain indexer to start collecting data. Choose the data type
                  you want to index (NFT prices, token prices, etc.) and specify which collection or tokens
                  you want to track.
                </p>
              </div>
            </div>
          </CardContent>

          <CardFooter className="flex justify-between border-t border-border/40 py-5 px-6 bg-muted/30">
            <Button
              variant="outline"
              onClick={() => navigate('/dashboard/db-credentials')}
            >
              View All Credentials
            </Button>
            <Button
              onClick={() => navigate('/dashboard/indexers/create')}
              className="flex items-center gap-2"
            >
              <BarChart3 className="h-4 w-4" />
              Create Indexer
            </Button>
          </CardFooter>
        </Card>
      </div>
    );
  }

  return (
    <div className="max-w-4xl mx-auto px-4">
      <motion.div
        initial="hidden"
        animate="visible"
        variants={containerVariants}
        className="mb-6"
      >
        <Button
          variant="ghost"
          onClick={() => navigate('/dashboard/db-credentials')}
          className="mb-4 text-muted-foreground hover:text-primary transition-colors"
        >
          <ArrowLeft className="h-4 w-4 mr-2" />
          Back to credentials
        </Button>

        <h1 className="text-3xl font-bold">{isEditing ? 'Edit Database Credential' : 'Add Database Credential'}</h1>
        <p className="text-muted-foreground mt-1">
          {isEditing
            ? 'Update your PostgreSQL database connection details'
            : 'Connect your PostgreSQL database to start indexing blockchain data'}
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
            <span className="text-sm font-medium">Connection Details</span>
          </div>

          <div className={`flex flex-col items-center ${step >= 2 ? 'text-primary' : 'text-muted-foreground'}`}>
            <div className={`h-10 w-10 rounded-full flex items-center justify-center mb-2 ${step >= 2 ? 'bg-primary text-white' : 'bg-muted'}`}>
              2
            </div>
            <span className="text-sm font-medium">Review & Save</span>
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

      {step === 1 && (
        <motion.div
          key="step1"
          variants={cardVariants}
          initial="hidden"
          animate="visible"
          exit="exit"
        >
          <Card className="border-border/60 shadow-lg hover:shadow-xl transition-all duration-300">
            <CardHeader className="bg-muted/30 border-b border-border/40 space-y-1">
              <div className="flex items-center gap-2">
                <Database className="h-5 w-5 text-primary" />
                <CardTitle>PostgreSQL Connection Details</CardTitle>
              </div>
              <CardDescription>
                Enter your database connection information to securely connect your PostgreSQL database
              </CardDescription>
            </CardHeader>

            <form onSubmit={(e) => { e.preventDefault(); testConnection(); }}>
              <CardContent className="space-y-6 pt-6">
                {error && (
                  <motion.div
                    className="bg-destructive/15 text-destructive text-sm p-4 rounded-md flex items-center border border-destructive/20"
                    initial={{ opacity: 0, height: 0 }}
                    animate={{ opacity: 1, height: 'auto' }}
                    transition={{ duration: 0.3 }}
                  >
                    <AlertCircle className="h-4 w-4 mr-2 flex-shrink-0" />
                    {error}
                  </motion.div>
                )}

                {/* Connection URL Input */}
                <motion.div
                  variants={itemVariants}
                  className="space-y-2 bg-muted/30 p-4 rounded-md border border-border/40"
                >
                  <label className="text-sm font-medium flex items-center gap-1.5" htmlFor="connectionUrl">
                    <LinkIcon className="h-4 w-4 text-primary" />
                    Connection URL
                  </label>
                  <div className="flex gap-2">
                    <Input
                      id="connectionUrl"
                      placeholder="postgresql://user:password@host:port/dbname?sslmode=require"
                      value={connectionUrl}
                      onChange={(e) => setConnectionUrl(e.target.value)}
                      className="flex-grow transition-all focus-visible:ring-1 focus-visible:ring-primary"
                    />
                    <Button
                      type="button"
                      onClick={parseConnectionUrl}
                      className="whitespace-nowrap"
                    >
                      Parse URL
                    </Button>
                  </div>
                  <p className="text-xs text-muted-foreground">
                    Enter a PostgreSQL connection URL to automatically fill in the form below
                  </p>
                </motion.div>

                {/* OR Divider */}
                <div className="relative flex items-center py-2">
                  <div className="flex-grow border-t border-border"></div>
                  <span className="flex-shrink mx-4 text-muted-foreground font-medium">OR</span>
                  <div className="flex-grow border-t border-border"></div>
                </div>

                <motion.div variants={itemVariants} className="grid md:grid-cols-2 gap-6">
                  <div className="space-y-2">
                    <label className="text-sm font-medium flex items-center gap-1.5" htmlFor="host">
                      <Server className="h-4 w-4 text-muted-foreground" />
                      Hostname <span className="text-destructive">*</span>
                    </label>
                    <Input
                      id="host"
                      name="host"
                      placeholder="localhost or 127.0.0.1"
                      value={formData.host}
                      onChange={handleChange}
                      className="transition-all focus-visible:ring-1 focus-visible:ring-primary"
                    />
                    <p className="text-xs text-muted-foreground">
                      The hostname or IP address of your PostgreSQL server
                    </p>
                  </div>

                  <div className="space-y-2">
                    <label className="text-sm font-medium flex items-center gap-1.5" htmlFor="port">
                      <Globe className="h-4 w-4 text-muted-foreground" />
                      Port <span className="text-destructive">*</span>
                    </label>
                    <Input
                      id="port"
                      name="port"
                      type="number"
                      placeholder="5432"
                      value={formData.port}
                      onChange={handleChange}
                      className="transition-all focus-visible:ring-1 focus-visible:ring-primary"
                    />
                    <p className="text-xs text-muted-foreground">
                      Default PostgreSQL port is 5432
                    </p>
                  </div>
                </motion.div>

                <motion.div variants={itemVariants} className="space-y-2">
                  <label className="text-sm font-medium flex items-center gap-1.5" htmlFor="name">
                    <Database className="h-4 w-4 text-muted-foreground" />
                    Database Name <span className="text-destructive">*</span>
                  </label>
                  <Input
                    id="name"
                    name="name"
                    placeholder="postgres"
                    value={formData.name}
                    onChange={handleChange}
                    className="transition-all focus-visible:ring-1 focus-visible:ring-primary"
                  />
                  <p className="text-xs text-muted-foreground">
                    The name of your PostgreSQL database
                  </p>
                </motion.div>

                <div className="border-t border-border/40 pt-4">
                  <h3 className="font-medium text-sm mb-4 flex items-center">
                    <ShieldCheck className="h-4 w-4 mr-2 text-muted-foreground" />
                    Authentication Details
                  </h3>

                  <motion.div variants={itemVariants} className="grid md:grid-cols-2 gap-6">
                    <div className="space-y-2">
                      <label className="text-sm font-medium flex items-center gap-1.5" htmlFor="user">
                        <Key className="h-4 w-4 text-muted-foreground" />
                        Username <span className="text-destructive">*</span>
                      </label>
                      <Input
                        id="user"
                        name="user"
                        placeholder="postgres"
                        value={formData.user}
                        onChange={handleChange}
                        className="transition-all focus-visible:ring-1 focus-visible:ring-primary"
                      />
                    </div>

                    <div className="space-y-2">
                      <label className="text-sm font-medium flex items-center gap-1.5" htmlFor="password">
                        <Lock className="h-4 w-4 text-muted-foreground" />
                        Password <span className="text-destructive">*</span>
                      </label>
                      <PasswordInput
                        id="password"
                        name="password"
                        placeholder={isEditing ? "Leave blank to keep current password" : "Enter password"}
                        value={formData.password}
                        onChange={handleChange}
                        required={!isEditing}
                        className="transition-all focus-visible:ring-1 focus-visible:ring-primary"
                      />
                    </div>
                  </motion.div>
                </div>

                <motion.div variants={itemVariants} className="border-t border-border/40 pt-4">
                  <div className="bg-muted/50 p-4 rounded-md mb-4">
                    <div className="flex">
                      <div className="flex-shrink-0">
                        <Info className="h-5 w-5 text-primary" />
                      </div>
                      <div className="ml-3">
                        <h3 className="text-sm font-medium">Connection Security</h3>
                        <div className="mt-1 text-sm text-muted-foreground">
                          <p>SSL mode determines how the connection to your database is secured. For development environments, 'disable' is often used, but for production, consider using 'require' or 'verify-full'.</p>
                        </div>
                      </div>
                    </div>
                  </div>

                  <div className="space-y-2">
                    <label className="text-sm font-medium flex items-center gap-1.5" htmlFor="sslMode">
                      <Tag className="h-4 w-4 text-muted-foreground" />
                      SSL Mode
                    </label>
                    <select
                      id="sslMode"
                      name="sslMode"
                      value={formData.sslMode}
                      onChange={(e: React.ChangeEvent<HTMLSelectElement>) => handleChange(e as unknown as React.ChangeEvent<HTMLInputElement>)}
                      className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm transition-all focus-visible:ring-1 focus-visible:ring-primary"
                    >
                      <option value="disable">disable - No SSL</option>
                      <option value="require">require - Always use SSL</option>
                      <option value="verify-ca">verify-ca - Verify server certificate</option>
                      <option value="verify-full">verify-full - Verify server certificate and hostname</option>
                    </select>
                  </div>
                </motion.div>
              </CardContent>

              <CardFooter className="flex justify-between border-t border-border/40 py-5 px-6 bg-muted/30">
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => navigate('/dashboard/db-credentials')}
                  className="relative overflow-hidden group"
                >
                  <span className="relative z-10">Cancel</span>
                  <span className="absolute inset-0 bg-background hover:bg-muted transition-colors duration-200"></span>
                </Button>
                <Button
                  type="submit"
                  disabled={isLoading}
                  className="relative overflow-hidden group"
                >
                  <span className="relative z-10 flex items-center">
                    {isLoading ? (
                      <>
                        <svg className="animate-spin -ml-1 mr-2 h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                        </svg>
                        Testing Connection...
                      </>
                    ) : 'Test Connection & Continue'}
                  </span>
                  <ChevronRight className="ml-2 h-4 w-4 relative z-10" />
                  <span className="absolute inset-0 bg-gradient-to-r from-primary/80 to-primary opacity-0 group-hover:opacity-100 transition-opacity duration-300"></span>
                </Button>
              </CardFooter>
            </form>
          </Card>
        </motion.div>
      )}

      {step === 2 && (
        <motion.div
          key="step2"
          variants={cardVariants}
          initial="hidden"
          animate="visible"
          exit="exit"
        >
          <Card className="border-border/60 shadow-lg hover:shadow-xl transition-all duration-300">
            <CardHeader className="bg-muted/30 border-b border-border/40 space-y-1">
              <div className="flex items-center gap-2">
                <Database className="h-5 w-5 text-primary" />
                <CardTitle>Review & Confirm</CardTitle>
              </div>
              <CardDescription>
                Review your PostgreSQL database connection details before saving
              </CardDescription>
            </CardHeader>

            <form onSubmit={handleSubmit}>
              <CardContent className="space-y-6 pt-6">
                {error && (
                  <motion.div
                    className="bg-destructive/15 text-destructive text-sm p-4 rounded-md flex items-center border border-destructive/20"
                    initial={{ opacity: 0, height: 0 }}
                    animate={{ opacity: 1, height: 'auto' }}
                    transition={{ duration: 0.3 }}
                  >
                    <AlertCircle className="h-4 w-4 mr-2 flex-shrink-0" />
                    {error}
                  </motion.div>
                )}

                <motion.div
                  variants={itemVariants}
                  className="p-4 rounded-lg border border-green-200 bg-green-50 dark:border-green-900/50 dark:bg-green-900/20 mb-6 flex items-start"
                >
                  <ShieldCheck className="h-5 w-5 text-green-600 dark:text-green-400 mt-0.5 mr-3 flex-shrink-0" />
                  <div>
                    <h3 className="font-medium text-green-800 dark:text-green-400">Connection Successful</h3>
                    <p className="text-sm text-green-700 dark:text-green-500 mt-1">Your database connection was tested successfully. You can now save this configuration.</p>
                  </div>
                </motion.div>

                {/* Connection Summary */}
                <div className="space-y-4">
                  <motion.h3
                    variants={itemVariants}
                    className="font-medium text-sm text-muted-foreground"
                  >
                    CONNECTION SUMMARY
                  </motion.h3>

                  <motion.div
                    variants={itemVariants}
                    className="grid md:grid-cols-2 gap-x-6 gap-y-4"
                  >
                    <div className="space-y-1">
                      <div className="text-xs font-medium text-muted-foreground">Hostname</div>
                      <div className="font-medium">{formData.host}</div>
                    </div>

                    <div className="space-y-1">
                      <div className="text-xs font-medium text-muted-foreground">Port</div>
                      <div className="font-medium">{formData.port}</div>
                    </div>

                    <div className="space-y-1">
                      <div className="text-xs font-medium text-muted-foreground">Database Name</div>
                      <div className="font-medium">{formData.name}</div>
                    </div>

                    <div className="space-y-1">
                      <div className="text-xs font-medium text-muted-foreground">Username</div>
                      <div className="font-medium">{formData.user}</div>
                    </div>

                    <div className="space-y-1">
                      <div className="text-xs font-medium text-muted-foreground">Password</div>
                      <div className="font-medium">••••••••</div>
                    </div>

                    <div className="space-y-1">
                      <div className="text-xs font-medium text-muted-foreground">SSL Mode</div>
                      <div className="font-medium">{formData.sslMode}</div>
                </div>
              </motion.div>
            </div>

            <motion.div
              variants={itemVariants}
              className="border rounded-md p-4 bg-muted/30"
            >
              <div className="flex justify-between items-center mb-2">
                <h3 className="font-medium">Connection String</h3>
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-8 gap-1.5 text-muted-foreground hover:text-foreground"
                  onClick={copyConnectionString}
                >
                  {isCopied ? (
                    <>
                      <Check className="h-3.5 w-3.5" />
                      Copied
                    </>
                  ) : (
                    <>
                      <Copy className="h-3.5 w-3.5" />
                      Copy
                    </>
                  )}
                </Button>
              </div>
              <div className="bg-muted p-3 rounded text-sm font-mono overflow-x-auto">
                postgresql://{formData.user}:******@{formData.host}:{formData.port}/{formData.name}
              </div>
            </motion.div>

            {!isEditing && (
              <motion.div
                variants={itemVariants}
                className="bg-primary/10 rounded-lg p-4 flex items-start gap-4"
              >
                <div className="flex-shrink-0 mt-1">
                  <Info className="h-5 w-5 text-primary" />
                </div>
                <div className="space-y-2">
                  <h4 className="font-medium">What's Next?</h4>
                  <p className="text-sm text-muted-foreground">
                    After saving this database credential, you'll be able to create blockchain indexers
                    to start collecting on-chain data into your PostgreSQL database.
                  </p>
                </div>
              </motion.div>
            )}
          </CardContent>

          <CardFooter className="flex justify-between border-t border-border/40 py-5 px-6 bg-muted/30">
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
            <Button
              type="submit"
              disabled={isLoading}
              className="relative overflow-hidden group"
            >
              <span className="relative z-10">
                {isLoading ? (
                  <span className="flex items-center">
                    <svg className="animate-spin -ml-1 mr-2 h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                    </svg>
                    {isEditing ? 'Updating...' : 'Creating...'}
                  </span>
                ) : isEditing ? 'Update Credential' : 'Save Credential'}
              </span>
              <span className="absolute inset-0 bg-gradient-to-r from-primary/80 to-primary opacity-0 group-hover:opacity-100 transition-opacity duration-300"></span>
            </Button>
          </CardFooter>
        </form>
          </Card>
        </motion.div >
      )}
    </div >
  );
};

export default DatabaseCredentialForm;

