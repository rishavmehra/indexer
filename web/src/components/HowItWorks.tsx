import { Card, CardContent, CardHeader, CardTitle } from "./ui/card";
import { CloudIcon, DatabaseIcon, FilterIcon, WebhookOffIcon } from "lucide-react";

interface FeatureProps {
  icon: JSX.Element;
  title: string;
  description: string;
}

const features: FeatureProps[] = [
  {
    icon: <DatabaseIcon />,
    title: "Connect Database",
    description: "Securely link your Postgres database with just a few clicks. Provide your credentials and get ready to index blockchain data effortlessly.",
  },
  {
    icon: <WebhookOffIcon />,
    title: "Helius Webhook Integration",
    description: "Leverage powerful Helius webhooks to stream real-time Solana blockchain data directly into your database without managing complex infrastructure.",
  },
  {
    icon: <FilterIcon />,
    title: "Customize Data Indexing",
    description: "Select from a range of predefined indexing categories like NFT bids, token prices, and borrowing availability. Tailor your data collection precisely.",
  },
  {
    icon: <CloudIcon />,
    title: "Automated Synchronization",
    description: "Our platform handles all backend complexities, continuously syncing and updating your database with the latest blockchain information.",
  },
];

export const HowItWorks = () => {
  return (
    <section
      id="howItWorks"
      className="container text-center py-24 sm:py-32"
    >
      <h2 className="text-3xl md:text-4xl font-bold ">
        How It{" "}
        <span className="bg-gradient-to-b from-primary/60 to-primary text-transparent bg-clip-text">
          Works{" "}
        </span>
        Step-by-Step Guide
      </h2>
      <p className="md:w-3/4 mx-auto mt-4 mb-8 text-xl text-muted-foreground">
        Simplify your Solana blockchain data indexing with our user-friendly platform. From database connection to real-time data retrieval, we've got you covered.
      </p>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-8">
        {features.map(({ icon, title, description }: FeatureProps) => (
          <Card
            key={title}
            className="bg-muted/50"
          >
            <CardHeader>
              <CardTitle className="grid gap-4 place-items-center">
                {icon}
                {title}
              </CardTitle>
            </CardHeader>
            <CardContent>{description}</CardContent>
          </Card>
        ))}
      </div>
    </section>
  );
};