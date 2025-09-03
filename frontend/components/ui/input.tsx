import * as React from "react";

import { cn } from "@/lib/utils";
import { Slot } from "@radix-ui/react-slot";
import { cva } from "class-variance-authority";

export const inputVariants = cva(
  "flex w-full rounded-full transition-all duration-300 ease-out backdrop-blur-lg bg-white/10 shadow-lg ring-1 ring-transparent focus-visible:bg-white/15 focus-visible:ring-1 focus-visible:ring-white/30 focus-visible:ring-offset-2 focus-visible:ring-offset-transparent disabled:cursor-not-allowed disabled:opacity-50 md:text-base text-white border border-white/20 h-11 !text-base placeholder:text-white/70 focus:outline-none px-4 hover:bg-white/15 hover:border-white/30",
);

const Input = React.forwardRef<HTMLInputElement, React.ComponentProps<"input"> & { asChild?: boolean }>(
  ({ className, type, asChild, ...props }, ref) => {
    const Comp = asChild ? Slot : "input";
    return (
      <Comp
        type={type}
        className={cn(inputVariants(), className)}
        ref={ref}
        {...props}
      />
    );
  }
);
Input.displayName = "Input";

export { Input };
