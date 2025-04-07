import React from 'react';
import IndexerForm from '@/components/indexers/IndexerForm';

const CreateIndexerPage: React.FC = () => {
  return (
    <div className="p-6">
      <h1 className="text-3xl font-bold mb-6">Create Blockchain Indexer</h1>
      <IndexerForm />
    </div>
  );
};

export default CreateIndexerPage;