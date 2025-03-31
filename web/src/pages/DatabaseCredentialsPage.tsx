import React from 'react';
import DatabaseCredentialsList from '@/components/dashboard/DatabaseCredentialsList';

const DatabaseCredentialsPage: React.FC = () => {
  return (
    <div className="p-6">
      <DatabaseCredentialsList />
    </div>
  );
};

export default DatabaseCredentialsPage;