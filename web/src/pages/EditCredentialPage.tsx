import { useParams } from 'react-router-dom';
import DatabaseCredentialForm from '@/components/dashboard/DatabaseCredentialForm';
import { motion } from 'framer-motion';

const EditCredentialPage = () => {
  const { id } = useParams();

  if (!id) {
    return <div>Invalid credential ID</div>;
  }

  return (
    <motion.div 
      className="p-6"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: 0.3 }}
    >
      <DatabaseCredentialForm credentialId={id} isEditing={true} />
    </motion.div>
  );
};

export default EditCredentialPage;