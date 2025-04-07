import React, { useState, useEffect } from 'react';
import { PasswordInput } from '@/components/PasswordInput';
import { Lock } from 'lucide-react';

interface PasswordFieldProps {
  value: string;
  onChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
  id: string;
  label: string;
  placeholder?: string;
  required?: boolean;
  confirmPassword?: string;
}

const PasswordField: React.FC<PasswordFieldProps> = ({
  value,
  onChange,
  id,
  label,
  placeholder,
  required = true,
  confirmPassword
}) => {
  const [validationErrors, setValidationErrors] = useState<string[]>([]);
  const [isTouched, setIsTouched] = useState(false);
  
  // Validate the password whenever it changes
  useEffect(() => {
    if (!isTouched) return;
    
    const errors: string[] = [];
    
    // Only show errors if the field has been touched
    if (value.length < 8) {
      errors.push("Password must be at least 8 characters long");
    }
    
    if (!/[a-zA-Z]/.test(value)) {
      errors.push("Password must include at least one letter");
    }
    
    if (!/[0-9]/.test(value)) {
      errors.push("Password must include at least one number");
    }
    
    // If this is a confirm password field
    if (confirmPassword !== undefined && value !== confirmPassword && value.length > 0) {
      errors.push("Passwords do not match");
    }
    
    setValidationErrors(errors);
  }, [value, confirmPassword, isTouched]);

  return (
    <div className="space-y-2">
      <label className="text-sm font-medium leading-none flex items-center gap-1.5" htmlFor={id}>
        <Lock className="h-4 w-4" />
        {label}
      </label>
      <div className="relative">
        <PasswordInput
          id={id}
          value={value}
          onChange={onChange}
          onBlur={() => setIsTouched(true)}
          className={`transition-all focus:ring-2 focus:ring-primary/20 ${
            isTouched && validationErrors.length > 0 ? 'border-destructive' : ''
          }`}
          placeholder={placeholder}
          required={required}
        />
      </div>
      
      {/* Error messages */}
      {isTouched && validationErrors.length > 0 && (
        <div className="space-y-1">
          {validationErrors.map((error, index) => (
            <p key={index} className="text-xs text-destructive flex items-center gap-1">
              <span className="h-1 w-1 rounded-full bg-destructive inline-block"></span>
              {error}
            </p>
          ))}
        </div>
      )}
    </div>
  );
};

export default PasswordField;