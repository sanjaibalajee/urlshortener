"use client";

import { useEffect, useRef, useState } from "react";
import { Button, buttonVariants } from "./ui/button";
import { AnimatePresence, motion } from "framer-motion";
import { cn } from "@/lib/utils";
import { ArrowRightIcon, Cross1Icon, Link1Icon as Link1Icon} from "@radix-ui/react-icons";
import { inputVariants } from "@/components/ui/input";

const DURATION = 0.3;
const DELAY = DURATION;
const EASE_OUT = "easeOut";
const EASE_OUT_OPACITY = [0.25, 0.46, 0.45, 0.94] as const;
const SPRING = {
  type: "spring" as const,
  stiffness: 60,
  damping: 10,
  mass: 0.8,
};

interface ShortenerInputProps {
  onSubmit?: (url: string) => void;
}

export const ShortenerInput = ({ onSubmit }: ShortenerInputProps) => {
  const [isOpen, setIsOpen] = useState(false);
  const [url, setUrl] = useState("");
  const [shortUrl, setShortUrl] = useState("");

  const isInitialRender = useRef(true);

  useEffect(() => {
    return () => {
      isInitialRender.current = false;
    };
  }, [isOpen]);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setIsOpen(false);
      }
    };

    window.addEventListener("keydown", handleKeyDown);

    return () => {
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, []);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (url.trim()) {
      onSubmit?.(url.trim());
      // Simulate URL shortening
      setShortUrl(`https://takeme.site/${Math.random().toString(36).substr(2, 8)}`);
    }
  };

  return (
    <div className="flex overflow-hidden relative flex-col gap-4 justify-center items-center pt-10 w-full h-full short:lg:pt-10 pb-footer-safe-area 2xl:pt-footer-safe-area px-sides short:lg:gap-4 lg:gap-8">
      <motion.div
        layout="position"
        transition={{ duration: DURATION, ease: EASE_OUT }}
      >
        <h1 className="font-serif text-5xl italic short:lg:text-8xl sm:text-8xl lg:text-9xl text-white">
          takeme.site
        </h1>
      </motion.div>

      <div className="flex flex-col items-center min-h-0 shrink">
        <AnimatePresence mode="popLayout" propagate>
          {!isOpen && (
            <motion.div
              key="shortener"
              initial={isInitialRender.current ? false : "hidden"}
              animate="visible"
              exit="exit"
              variants={{
                visible: {
                  scale: 1,
                  transition: {
                    delay: DELAY,
                    duration: DURATION,
                    ease: EASE_OUT,
                  },
                },
                hidden: {
                  scale: 0.9,
                  transition: { duration: DURATION, ease: EASE_OUT },
                },
                exit: {
                  y: -150,
                  scale: 0.9,
                  transition: { duration: DURATION, ease: EASE_OUT },
                },
              }}
            >
              <div className="flex flex-col gap-4 w-full max-w-xl md:gap-6 lg:gap-8">
                <form onSubmit={handleSubmit} className="flex gap-2">
                  <motion.input
                    type="url"
                    value={url}
                    onChange={(e) => setUrl(e.target.value)}
                    autoCapitalize="off"
                    autoComplete="url"
                    placeholder="Enter URL to shorten"
                    className={inputVariants({ className: "flex-1" })}
                    initial={isInitialRender.current ? false : { opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{
                      opacity: 0,
                      transition: {
                        duration: DURATION,
                        ease: EASE_OUT_OPACITY,
                      },
                    }}
                    transition={{
                      duration: DURATION,
                      ease: EASE_OUT,
                      delay: DELAY,
                    }}
                  />
                  <motion.button
                    type="submit"
                    className={buttonVariants({
                      variant: "glass",
                      size: "default",
                    })}
                    initial={isInitialRender.current ? false : { opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{
                      opacity: 0,
                      transition: {
                        duration: DURATION,
                        ease: EASE_OUT_OPACITY,
                      },
                    }}
                    transition={{
                      duration: DURATION,
                      ease: EASE_OUT,
                      delay: DELAY,
                    }}
                  >
                    <ArrowRightIcon className="w-4 h-4" />
                    Shorten
                  </motion.button>
                </form>
                
                {shortUrl && (
                  <motion.div
                    initial={{ opacity: 0, y: 20 }}
                    animate={{ opacity: 1, y: 0 }}
                    className="p-4 backdrop-blur-lg bg-white/10 border border-white/20 rounded-lg shadow-lg"
                  >
                    <p className="text-sm text-white/80 mb-2">Shortened URL:</p>
                    <div className="flex items-center gap-2">
                      <code className="flex-1 px-2 py-1 bg-black/20 border border-white/10 rounded text-sm text-white backdrop-blur-sm">
                        {shortUrl}
                      </code>
                      <Button
                        size="sm"
                        variant="glass"
                        onClick={() => navigator.clipboard.writeText(shortUrl)}
                      >
                        Copy
                      </Button>
                    </div>
                  </motion.div>
                )}

                <motion.p
                  initial={isInitialRender.current ? false : { opacity: 0 }}
                  animate={{ opacity: 1 }}
                  exit={{
                    opacity: 0,
                    transition: { duration: DURATION, ease: EASE_OUT_OPACITY },
                  }}
                  transition={{
                    duration: DURATION,
                    ease: EASE_OUT,
                    delay: DELAY,
                  }}
                  className="text-base short:lg:text-lg sm:text-lg lg:text-xl !leading-[1.1] font-medium text-center text-white text-pretty"
                >
                  Transform long URLs into short, shareable links instantly.
                  Perfect for social media, emails, and tracking clicks.
                </motion.p>
              </div>
            </motion.div>
          )}


        </AnimatePresence>
      </div>
    </div>
  );
};