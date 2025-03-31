import { buttonVariants } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Linkedin, Github } from "lucide-react";

interface TeamProps {
  imageUrl: string;
  name: string;
  position: string;
  socialNetworks: SocialNetworkProps[];
}

interface SocialNetworkProps {
  name: string;
  url: string;
}

const teamList: TeamProps[] = [
  {
    imageUrl: "https://github.com/rishavmehra.png",
    name: "Rishav Mehra",
    position: "Founder & Lead Developer | Full time DevOps Engineer",
    socialNetworks: [
      {
        name: "Linkedin",
        url: "https://www.linkedin.com/in/rishavmehra/",
      },
      {
        name: "Github",
        url: "https://github.com/rishavmehra",
      },
    ],
  },
];

export const Team = () => {
  const socialIcon = (iconName: string) => {
    switch (iconName) {
      case "Linkedin":
        return <Linkedin size="20" />;
      case "Github":
        return <Github size="20" />;
    }
  };

  return (
    <section
      id="team"
      className="container py-24 sm:py-32 flex flex-col items-center"
    >
      <h2 className="text-3xl md:text-4xl font-bold mb-10">
        <span className="bg-gradient-to-b from-primary/60 to-primary text-transparent bg-clip-text">
          Project{" "}
        </span>
        Creator
      </h2>

      <div className="w-full max-w-md flex justify-center">
        {teamList.map(
          ({ imageUrl, name, position, socialNetworks }: TeamProps) => (
            <Card
              key={name}
              className="bg-muted/50 relative mt-8 flex flex-col justify-center items-center w-full"
            >
              <CardHeader className="mt-8 flex justify-center items-center pb-2">
                <img
                  src={imageUrl}
                  alt={`${name} ${position}`}
                  className="absolute -top-12 rounded-full w-24 h-24 aspect-square object-cover"
                />
                <CardTitle className="text-center mt-12">{name}</CardTitle>
                <CardDescription className="text-primary">
                  {position}
                </CardDescription>
              </CardHeader>

              <CardContent className="text-center pb-2">
                <p>Blockchain indexing platform developer passionate about simplifying web3 data integration.</p>
              </CardContent>

              <CardFooter className="flex justify-center gap-2">
                {socialNetworks.map(({ name, url }: SocialNetworkProps) => (
                  <a
                    key={name}
                    rel="noreferrer noopener"
                    href={url}
                    target="_blank"
                    className={buttonVariants({
                      variant: "ghost",
                      size: "sm",
                    })}
                  >
                    <span className="sr-only">{name} icon</span>
                    {socialIcon(name)}
                  </a>
                ))}
              </CardFooter>
            </Card>
          )
        )}
      </div>
    </section>
  );
};