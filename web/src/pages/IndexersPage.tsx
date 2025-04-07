import React from 'react';
import IndexerList from '@/components/indexers/IndexerList';
import { motion } from 'framer-motion';

const IndexersPage: React.FC = () => {
  return (
    <motion.div 
      className="p-6"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: 0.3 }}
    >
      <IndexerList />
    </motion.div>
  );
};

export default IndexersPage;