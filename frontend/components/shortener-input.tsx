"use client";

import { useEffect, useRef, useState } from "react";
import { Button, buttonVariants } from "./ui/button";
import { AnimatePresence, motion } from "framer-motion";
import { cn } from "@/lib/utils";
import { ArrowRightIcon, Cross1Icon, Link1Icon as Link1Icon} from "@radix-ui/react-icons";
import { inputVariants } from "@/components/ui/input";
import { shortenUrl } from "@/app/actions/url-services";

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
  const [customCode, setCustomCode] = useState("");
  const [showCustomCode, setShowCustomCode] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");

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

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!url.trim()) return;
    
    setIsLoading(true);
    setError("");
    setShortUrl("");
    
    try {
      const request = {
        url: url.trim(),
        ...(customCode.trim() && { custom_code: customCode.trim() })
      };
      
      const result = await shortenUrl(request);
      
      if (result.success) {
        setShortUrl(`https://takeme.site/${result.data.short_code}`);
        onSubmit?.(url.trim());
      } else {
        setError(result.message);
      }
    } catch (err) {
      setError("Failed to shorten URL. Please try again.");
    } finally {
      setIsLoading(false);
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
                <form onSubmit={handleSubmit} className="flex flex-col gap-3">
                  <div className="flex gap-2">
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
                      required
                      disabled={isLoading}
                    />
                    <motion.button
                      type="submit"
                      disabled={isLoading || !url.trim()}
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
                      {isLoading ? "Shortening..." : "Shorten"}
                    </motion.button>
                  </div>
                  
                  <AnimatePresence>
                    {showCustomCode && (
                      <motion.input
                        type="text"
                        value={customCode}
                        onChange={(e) => setCustomCode(e.target.value)}
                        placeholder="Enter custom code (optional)"
                        className={inputVariants({ className: "w-full" })}
                        initial={{ opacity: 0, height: 0, marginTop: 0 }}
                        animate={{ opacity: 1, height: "auto", marginTop: 12 }}
                        exit={{ 
                          opacity: 0, 
                          height: 0, 
                          marginTop: 0,
                          transition: {
                            duration: DURATION,
                            ease: EASE_OUT_OPACITY,
                          }
                        }}
                        transition={{
                          duration: DURATION,
                          ease: EASE_OUT,
                        }}
                        disabled={isLoading}
                        autoFocus
                      />
                    )}
                  </AnimatePresence>
                  
                  {!showCustomCode && (
                    <motion.button
                      type="button"
                      onClick={() => setShowCustomCode(true)}
                      className="text-sm text-white/60 hover:text-white/80 transition-colors self-start"
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
                        delay: DELAY + 0.1,
                      }}
                    >
                      need to use a custom code?
                    </motion.button>
                  )}
                </form>
                
                {error && (
                  <motion.div
                    initial={{ opacity: 0, y: 20 }}
                    animate={{ opacity: 1, y: 0 }}
                    className="p-4 backdrop-blur-lg bg-red-500/10 border border-red-500/20 rounded-lg shadow-lg"
                  >
                    <p className="text-sm text-red-300">{error}</p>
                  </motion.div>
                )}
                
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
                  transform long urls into short, shareable links instantly.
                  perfect for social media, emails, and tracking clicks.
                </motion.p>
              </div>
            </motion.div>
          )}


        </AnimatePresence>
      </div>
    </div>
  );
};