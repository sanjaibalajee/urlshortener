import { GitHubLogoIcon, LinkedInLogoIcon } from "@radix-ui/react-icons";
import { buttonVariants } from "./ui/button";
import XLogoIcon from "./Icons/x";
import { socialLinks } from "@/lib/constants";
import Link from "next/link";

export const Footer = () => {
  return (
    <div className="flex gap-4 items-center justify-center pb-8 px-4">
      <Link 
        target="_blank" 
        className={buttonVariants({ 
          variant: "glass", 
          size: "icon-xl" 
        })} 
        href={socialLinks.linkedin}
      >
        <LinkedInLogoIcon className="size-6" />
      </Link>
      <Link 
        target="_blank" 
        className={buttonVariants({ 
          variant: "glass", 
          size: "icon-xl" 
        })} 
        href={socialLinks.x}
      >
        <XLogoIcon className="size-6" />
      </Link>
      <Link 
        target="_blank" 
        className={buttonVariants({ 
          variant: "glass", 
          size: "icon-xl" 
        })} 
        href={socialLinks.github}
      >
        <GitHubLogoIcon className="size-6" />
      </Link>
    </div>
  );
};