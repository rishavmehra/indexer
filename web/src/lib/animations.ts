import { Variants } from 'framer-motion';

// Fade in animation
export const fadeIn = (delay: number = 0): Variants => ({
  hidden: { 
    opacity: 0 
  },
  visible: { 
    opacity: 1,
    transition: {
      delay,
      duration: 0.5
    }
  }
});

// Fade up animation
export const fadeUp = (delay: number = 0, y: number = 20): Variants => ({
  hidden: { 
    opacity: 0, 
    y 
  },
  visible: { 
    opacity: 1, 
    y: 0,
    transition: {
      delay,
      duration: 0.5
    }
  }
});

// Fade down animation
export const fadeDown = (delay: number = 0, y: number = -20): Variants => ({
  hidden: { 
    opacity: 0, 
    y 
  },
  visible: { 
    opacity: 1, 
    y: 0,
    transition: {
      delay,
      duration: 0.5
    }
  }
});

// Staggered children animation
export const staggerContainer = (staggerChildren: number = 0.1): Variants => ({
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: {
      staggerChildren
    }
  }
});

// Scale animation
export const scaleUp = (delay: number = 0): Variants => ({
  hidden: { 
    opacity: 0,
    scale: 0.8
  },
  visible: { 
    opacity: 1,
    scale: 1,
    transition: {
      delay,
      duration: 0.5
    }
  }
});

// Card hover animation
export const cardHover = {
  rest: { 
    scale: 1,
    boxShadow: "0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06)",
    transition: {
      duration: 0.2,
      ease: "easeInOut"
    }
  },
  hover: { 
    scale: 1.02,
    boxShadow: "0 10px 15px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -2px rgba(0, 0, 0, 0.05)",
    transition: {
      duration: 0.2,
      ease: "easeInOut"
    }
  }
};

// Button hover animation
export const buttonHover = {
  rest: { 
    scale: 1,
    transition: {
      duration: 0.2,
      ease: "easeInOut"
    }
  },
  hover: { 
    scale: 1.05,
    transition: {
      duration: 0.2,
      ease: "easeInOut"
    }
  }
};

// Subtle pulse animation for cards or elements that need attention
export const pulse = {
  animate: {
    scale: [1, 1.02, 1],
    transition: {
      duration: 1.5,
      ease: "easeInOut",
      times: [0, 0.5, 1],
      repeat: Infinity,
      repeatType: "reverse"
    }
  }
};

// Page transition
export const pageTransition = {
  initial: { opacity: 0, y: 5 },
  animate: { 
    opacity: 1, 
    y: 0,
    transition: {
      duration: 0.3,
      ease: "easeInOut"
    }
  },
  exit: { 
    opacity: 0, 
    y: 5,
    transition: {
      duration: 0.2,
      ease: "easeInOut"
    }
  }
};