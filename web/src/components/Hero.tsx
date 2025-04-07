import { Button } from "./ui/button";
import { buttonVariants } from "./ui/button";
import { HeroCards } from "./HeroCards";
import { GitHubLogoIcon } from "@radix-ui/react-icons";
import { Link } from "react-router-dom";
import { useAuth } from "@/context/AuthContext";
import { motion } from "framer-motion";
import { staggerContainer, fadeUp} from "@/lib/animations";

export const Hero = () => {
  const { isAuthenticated } = useAuth();

  return (
    <section className="container grid lg:grid-cols-2 place-items-center py-20 md:py-32 gap-10 relative overflow-hidden">
      <motion.div 
        className="text-center lg:text-start space-y-6"
        initial="hidden"
        whileInView="visible"
        viewport={{ once: true }}
        variants={staggerContainer(0.2)}
      >
        <motion.main 
          className="text-5xl md:text-6xl font-bold"
          variants={fadeUp(0.2)}
        >
          <h1 className="inline">
            <span className="inline bg-gradient-to-r from-[#f54c3f] to-[#ea7e78] text-transparent bg-clip-text animate-gradient">
              From Blockchain 
            </span>{" "}
              To Postgres
          </h1>{" "}
          {" "}
          <h2 className="inline">
            <span className="inline bg-gradient-to-r from-[#39fb40] via-[#1fc0f1] to-[#74ec78] text-transparent bg-clip-text animate-gradient">
            With Ease 
            </span>{" "}
          </h2>
        </motion.main>

        <motion.p 
          className="text-xl text-muted-foreground md:w-10/12 mx-auto lg:mx-0"
          variants={fadeUp(0.4)}
        >
          Fast and scalable blockchain indexing platform that enables developers to seamlessly integrate and query on-chain data in real-time using PostgreSQL.
        </motion.p>

        <motion.div 
          className="space-y-4 md:space-y-0 md:space-x-4"
          variants={fadeUp(0.6)}
        >
          <Link to={isAuthenticated ? "/dashboard" : "/login"}>
            <Button className="w-full md:w-1/3 group relative overflow-hidden">
              <span className="relative z-10">Get Started</span>
              <span className="absolute inset-0 bg-gradient-to-r from-primary/80 to-primary opacity-0 group-hover:opacity-100 transition-opacity duration-300"></span>
            </Button>
          </Link>

          <a
            rel="noreferrer noopener"
            href="https://github.com/rishavmehra/indexer"
            target="_blank"
            className={`w-full md:w-1/3 ${buttonVariants({
              variant: "outline",
            })} group`}
          >
            <span className="relative z-10 flex items-center">
              Github Repository
              <GitHubLogoIcon className="ml-2 w-5 h-5 group-hover:rotate-12 transition-transform duration-300" />
            </span>
          </a>
        </motion.div>
      </motion.div>

      {/* Hero cards sections */}
      <motion.div 
        className="z-10"
        initial={{ opacity: 0, x: 20 }}
        animate={{ opacity: 1, x: 0 }}
        transition={{ duration: 0.7, delay: 0.3 }}
      >
        <HeroCards />
      </motion.div>

      {/* Animated background elements */}
      <motion.div 
        className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-full h-full max-w-7xl"
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={{ duration: 1, delay: 0.5 }}
      >
        <div className="absolute top-1/4 right-1/4 w-72 h-72 bg-primary/10 rounded-full blur-3xl"></div>
        <div className="absolute bottom-1/4 left-1/3 w-64 h-64 bg-blue-500/10 rounded-full blur-3xl"></div>
      </motion.div>

      {/* Shadow effect */}
      <div className="shadow"></div>
    </section>
  );
};