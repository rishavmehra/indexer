import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card, CardContent, CardFooter, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { AlertCircle, Database, Server, Globe, Key, Lock, Tag, ArrowLeft, ChevronRight, Info, ShieldCheck, Copy, Check } from 'lucide-react';
import { DBCredentialService } from '@/services/api';
import { motion } from 'framer-motion';
import { PasswordInput } from '@/components/PasswordInput';
import { useToast } from '@/components/ui/toast';

const DatabaseCredentialForm = ({ credentialId , isEditing = false }: { credentialId: string, isEditing: boolean }) => {
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
  // @ts-ignore
  const [isConnectionTested, setIsConnectionTested] = useState(false);
  const [isCopied, setIsCopied] = useState(false);
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

  const testConnection = async () => {
    setIsLoading(true);
    setError('');
    
    if (!formData.host || !formData.port || !formData.name || !formData.user || (!isEditing && !formData.password)) {
      setError('Please fill in all required fields');
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
        setIsConnectionTested(true);
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
      } else {
        await DBCredentialService.create(formData);
        addToast({
          title: 'Credential Created',
          description: 'New database credential has been created successfully.',
          type: 'success'
        });
      }
      navigate('/dashboard/db-credentials');
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

  // Animation variants
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
                      required
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
                      required
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
                    required
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
                        required
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
        </motion.div>
      )}
    </div>
  );
};

export default DatabaseCredentialForm;
