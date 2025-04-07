import React, { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { AuthService } from '../services/api';
import { useAuth } from '../context/AuthContext';
import { 
  Card, 
  CardContent, 
  CardDescription, 
  CardFooter, 
  CardHeader, 
  CardTitle 
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { LogoIcon } from '@/components/Icons';
import { AlertCircle, Mail, Lock } from 'lucide-react';
import { motion } from 'framer-motion';
import { PasswordInput } from '@/components/PasswordInput';

const LoginPage: React.FC = () => {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');
  
  const { login } = useAuth();
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError('');
    
    try {
      const response = await AuthService.login(email, password);
      login(response.data.token, response.data);
      navigate('/dashboard');
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to login. Please try again.');
    } finally {
      setIsLoading(false);
    }
  };

  const containerVariants = {
    hidden: { opacity: 0 },
    visible: {
      opacity: 1,
      transition: {
        duration: 0.5,
        staggerChildren: 0.1,
        delayChildren: 0.2
      }
    }
  };

  const itemVariants = {
    hidden: { opacity: 0, y: 20 },
    visible: { opacity: 1, y: 0 }
  };

  const backgroundVariants = {
    hidden: { opacity: 0 },
    visible: { opacity: 1, transition: { duration: 0.8 }}
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-4 relative overflow-hidden">
      {/* Animated background elements */}
      <motion.div 
        className="absolute inset-0 -z-10"
        initial="hidden"
        animate="visible"
        variants={backgroundVariants}
      >
        <motion.div 
          className="absolute top-1/4 right-1/4 w-72 h-72 rounded-full bg-primary/5 blur-3xl"
          animate={{ 
            scale: [1, 1.1, 1],
            opacity: [0.3, 0.2, 0.3],
          }}
          transition={{ 
            duration: 5,
            repeat: Infinity,
            repeatType: "reverse"
          }}
        />
        <motion.div 
          className="absolute bottom-1/4 left-1/3 w-64 h-64 rounded-full bg-primary/5 blur-3xl"
          animate={{ 
            scale: [1, 1.2, 1],
            opacity: [0.2, 0.3, 0.2],
          }}
          transition={{ 
            duration: 6,
            repeat: Infinity,
            repeatType: "reverse",
            delay: 1
          }}
        />
      </motion.div>

      <motion.div
        className="w-full max-w-md"
        initial="hidden"
        animate="visible"
        variants={containerVariants}
      >
        <Card className="w-full backdrop-blur-sm bg-card/95 border-primary/5 shadow-xl overflow-hidden">
          <CardHeader className="space-y-1 items-center text-center relative">
            <motion.div
              className="absolute inset-0 bg-primary/5 -z-10"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ delay: 0.3, duration: 0.5 }}
            />
            <motion.div 
              className="flex items-center justify-center mb-2"
              variants={itemVariants}
            >
              <LogoIcon />
              <span className="ml-2 text-2xl font-bold">Indexer Pro</span>
            </motion.div>
            <motion.div variants={itemVariants}>
              <CardTitle className="text-2xl">Sign in to your account</CardTitle>
            </motion.div>
            <motion.div variants={itemVariants}>
              <CardDescription>
                Enter your credentials to access your account
              </CardDescription>
            </motion.div>
          </CardHeader>
          <form onSubmit={handleSubmit}>
            <CardContent className="space-y-4">
              {error && (
                <motion.div 
                  className="bg-destructive/15 text-destructive text-sm p-3 rounded-md flex items-center"
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: 'auto' }}
                  transition={{ duration: 0.3 }}
                >
                  <AlertCircle className="h-4 w-4 mr-2 flex-shrink-0" />
                  {error}
                </motion.div>
              )}
              <motion.div variants={itemVariants} className="space-y-2">
                <label className="text-sm font-medium leading-none flex items-center gap-1.5" htmlFor="email">
                  <Mail className="h-4 w-4" />
                  Email
                </label>
                <div className="relative">
                  <Input
                    id="email"
                    type="email"
                    placeholder="m@example.com"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    className="transition-all focus:ring-2 focus:ring-primary/20"
                    required
                  />
                </div>
              </motion.div>
              <motion.div variants={itemVariants} className="space-y-2">
                <div className="flex items-center justify-between">
                  <label className="text-sm font-medium leading-none flex items-center gap-1.5" htmlFor="password">
                    <Lock className="h-4 w-4" />
                    Password
                  </label>
                  <Link to="/forgot-password" className="text-sm text-primary hover:underline">
                    Forgot password?
                  </Link>
                </div>
                <div className="relative">
                  <PasswordInput
                    id="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    className="transition-all focus:ring-2 focus:ring-primary/20"
                    required
                  />
                </div>
              </motion.div>
            </CardContent>
            <CardFooter className="flex flex-col space-y-4">
              <motion.div 
                className="w-full"
                variants={itemVariants}
              >
                <Button 
                  type="submit" 
                  className="w-full relative overflow-hidden group" 
                  disabled={isLoading}
                >
                  <span className="relative z-10">
                    {isLoading ? (
                      <span className="flex items-center">
                        <svg className="animate-spin -ml-1 mr-2 h-4 w-4 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                        </svg>
                        Signing in...
                      </span>
                    ) : 'Sign In'}
                  </span>
                  <span className="absolute inset-0 bg-gradient-to-r from-primary/80 to-primary opacity-0 group-hover:opacity-100 transition-opacity duration-300"></span>
                </Button>
              </motion.div>
              
              <motion.p 
                className="text-center text-sm text-muted-foreground"
                variants={itemVariants}
              >
                Don't have an account?{' '}
                <Link to="/signup" className="text-primary underline-offset-4 hover:underline">
                  Sign up
                </Link>
              </motion.p>
            </CardFooter>
          </form>
        </Card>
      </motion.div>
    </div>
  );
};

export default LoginPage;

