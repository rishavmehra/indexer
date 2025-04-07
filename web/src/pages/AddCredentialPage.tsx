import DatabaseCredentialForm from '@/components/dashboard/DatabaseCredentialForm';
import { motion } from 'framer-motion';

const AddCredentialPage = () => {
  return (
    <motion.div 
      className="p-6"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: 0.3 }}
    >
      <DatabaseCredentialForm credentialId={''} isEditing={false} />
    </motion.div>
  );
};

export default AddCredentialPage;