import { Hero } from "../components/Hero";
import { HowItWorks } from "../components/HowItWorks";
import { Services } from "../components/Services";
import { Team } from "../components/Team";
import { FAQ } from "../components/FAQ";

const HomePage = () => {
  return (
    <>
      <Hero />
      <HowItWorks />
      <Services />
      <Team />
      <FAQ />
    </>
  );
};

export default HomePage;