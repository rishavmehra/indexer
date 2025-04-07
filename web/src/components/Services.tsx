import { Card, CardDescription, CardHeader, CardTitle } from "./ui/card";
import { DatabaseIcon, WebhookIcon, FilterIcon } from "lucide-react";
import blockchainIndexing from "../assets/pilot.png";

interface ServiceProps {
  title: string;
  description: string;
  icon: JSX.Element;
}

const serviceList: ServiceProps[] = [
  {
    title: "Real-Time Blockchain Indexing",
    description: "Seamlessly capture and store Solana blockchain data in your Postgres database with our advanced Helius webhook integration.",
    icon: <DatabaseIcon />,
  },
  {
    title: "Customizable Data Filtering",
    description: "Select from a wide range of predefined indexing categories, including NFT bids, token prices, and lending market data.",
    icon: <FilterIcon />,
  },
  {
    title: "Automated Data Synchronization",
    description: "Eliminate the complexity of managing blockchain infrastructure with our fully automated, real-time data retrieval system.",
    icon: <WebhookIcon />,
  },
];

export const Services = () => {
  return (
    <section id="services" className="container py-24 sm:py-32">
      <div className="grid lg:grid-cols-[1fr,1fr] gap-8 place-items-center">
        <div>
          <h2 className="text-3xl md:text-4xl font-bold">
            <span className="bg-gradient-to-b from-primary/60 to-primary text-transparent bg-clip-text">
              Blockchain{" "}
            </span>
            Indexing Solutions
          </h2>

          <p className="text-muted-foreground text-xl mt-4 mb-8">
            Simplify your blockchain data integration with our powerful, user-friendly platform designed for developers.
          </p>

          <div className="flex flex-col gap-8">
            {serviceList.map(({ icon, title, description }: ServiceProps) => (
              <Card key={title}>
                <CardHeader className="space-y-1 flex md:flex-row justify-start items-start gap-4">
                  <div className="mt-1 bg-primary/20 p-1 rounded-2xl">
                    {icon}
                  </div>
                  <div>
                    <CardTitle>{title}</CardTitle>
                    <CardDescription className="text-md mt-2">
                      {description}
                    </CardDescription>
                  </div>
                </CardHeader>
              </Card>
            ))}
          </div>
        </div>

        <img
          src={blockchainIndexing}
          className="w-[300px] md:w-[500px] lg:w-[600px] object-contain"
          alt="Blockchain Indexing Visualization"
        />
      </div>
    </section>
  );
};