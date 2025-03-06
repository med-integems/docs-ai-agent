"use client";

import { motion } from "framer-motion";
import Image from "next/image";
import { useRouter } from "next/navigation";
import { Button } from "./ui/button";
import { useCurrentUser } from "@/hooks/use-current-user";

// Animation variants for parent container
const containerVariant = {
  initial: {
    opacity: 0,
  },
  animate: {
    opacity: 1,
    transition: {
      staggerChildren: 1, // Stagger children animations by 0.2 seconds
    },
  },
};

// Animation variants for individual items
const childVariant = {
  initial: {
    opacity: 0,
    y: 20,
  },
  animate: {
    opacity: 1,
    y: 0,
    transition: {
      duration: 0.5,
      ease: "easeOut",
    },
  },
};

export default function Banner() {
  const user = useCurrentUser();
  const router = useRouter();

  const handleGetStarted = () => {
    if (user.isAuthenticated) {
      router.push(`/chat/${user.user?.userId}`);
    } else {
      router.push("/signin");
    }
  };

  return (
    <section className="max-w-7xl mx-auto px-4  md:px-20 mb-20">
      <motion.div
        className="flex justify-center items-center h-full"
        variants={containerVariant}
        initial="initial"
        animate="animate"
      >
        {/* Left Section */}
        <motion.div
          className="mx-auto w-full flex flex-col justify-center items-center "
          variants={containerVariant}
        >
          <div className="relative max-w-md w-full aspect-square mt-[10vh] mb-10">
            <Image
              alt="Chat bot image"
              fill
              className="absolute"
              src={"/chat-bot.gif"}
            />
          </div>
          <motion.h2
            className="text-bold font-extrabold text-3xl font-sans"
            variants={childVariant}
          >
            Optimize your <span className="text-primary">Document</span>
          </motion.h2>
          <motion.h2
            className="text-center my-2 text-lg text-slate-900 font-sans"
            variants={childVariant}
          >
            Write standard and format report efficienty,accurately, and
            seamlessly.
          </motion.h2>
          <Button onClick={handleGetStarted} className="m-2">
            Get started
          </Button>
        </motion.div>
      </motion.div>
    </section>
  );
}
