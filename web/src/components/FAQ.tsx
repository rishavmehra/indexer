import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";

interface FAQProps {
  question: string;
  answer: string;
  value: string;
}

const FAQList: FAQProps[] = [
  {
    question: "What is the story behind this blockchain indexing platform?",
    answer: "This platform was created by Rishav Mehra to solve the complex challenges developers face when integrating blockchain data. Recognizing the difficulties in managing RPC infrastructure and data indexing, Rishav developed this solution to simplify blockchain data integration for developers working with the Solana ecosystem.",
    value: "item-1",
  },
  {
    question: "How do I connect my Postgres database to the platform?",
    answer: "Our platform provides a simple, secure interface where you can input your Postgres database credentials. We use encrypted connections to ensure your data remains protected throughout the indexing process.",
    value: "item-2",
  },
  {
    question: "What types of blockchain data can I index?",
    answer: "We offer multiple predefined indexing categories, including NFT bids, token prices, lending market data, transaction histories, and more. You can select specific data points that are most relevant to your project.",
    value: "item-3",
  },
  {
    question: "Is there a limit to the amount of data I can index?",
    answer: "Our platform is designed to be scalable. While we have basic tiers for individual developers, we offer custom solutions for projects with high-volume data requirements. Contact our sales team for specific scaling options.",
    value: "item-4",
  },
  {
    question: "How often is the blockchain data updated?",
    answer: "Using Helius webhooks, we provide real-time data synchronization. This means your Postgres database is continuously updated with the latest blockchain information, ensuring you always have the most current data.",
    value: "item-5",
  },
];

export const FAQ = () => {
  return (
    <section
      id="faq"
      className="container py-24 sm:py-32"
    >
      <h2 className="text-3xl md:text-4xl font-bold mb-4">
        Frequently Asked{" "}
        <span className="bg-gradient-to-b from-primary/60 to-primary text-transparent bg-clip-text">
          Questions
        </span>
      </h2>

      <Accordion
        type="single"
        collapsible
        className="w-full AccordionRoot"
      >
        {FAQList.map(({ question, answer, value }: FAQProps) => (
          <AccordionItem
            key={value}
            value={value}
          >
            <AccordionTrigger className="text-left">
              {question}
            </AccordionTrigger>

            <AccordionContent>{answer}</AccordionContent>
          </AccordionItem>
        ))}
      </Accordion>

      <h3 className="font-medium mt-4">
        Still have questions?{" "}
                  <a
          rel="noreferrer noopener"
          href="https://x.com/rishavmehraa"
          className="text-primary transition-all border-primary hover:border-b-2"
        >
          Contact Rishav
        </a>
      </h3>
    </section>
  );
};