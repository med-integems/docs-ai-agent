"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { useToast } from "@/hooks/use-toast";
import { ArrowLeft } from "lucide-react";
import Image from "next/image";

export default function EmailEntryForm() {
  const [email, setEmail] = useState("");
  const [loading, setLoading] = useState(false);
  const { toast } = useToast();
  const router = useRouter();

  const handleSubmit = async () => {
    setLoading(true);
    if (!email) {
      toast({
        description: <p className="text-red-500">Email is required</p>,
      });
      setLoading(false);
      return;
    }

    try {
      const response = await fetch(
        `http://127.0.0.1:5000/check-email/${email}`,
        {
          method: "GET",
        },
      );

      const data = await response.json();

      if (!response.ok) {
        throw new Error(data.error || "Email does not exist");
      }

      toast({
        description: (
          <p className="text-green-500">Email verified! Redirecting...</p>
        ),
      });

      console.log({ Data: data });

      // Redirect user to new-password page with userId
      router.push(`/new-password?userId=${data.userId}`);
    } catch {
      // console.error(error);
      toast({
        description: (
          <p className="text-red-500">
            Couldn&apos;t verify. Ensure you entered a valid email.
          </p>
        ),
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="max-w-md mt-[10vh] mx-auto p-6 bg-white rounded-lg shadow-md">
      <Button
        variant="ghost"
        size="icon"
        className="text-primary font-bold"
        onClick={() => router.back()}
      >
        <ArrowLeft strokeWidth={3} />
      </Button>
      <div className="relative w-full m:max-w-[15vw] md:w-[15vw] mx-auto aspect-square">
        <Image
          alt="Login image"
          fill
          className="absolute"
          src={"/emails.gif"}
        />
      </div>
      <div className="space-y-4">
        <div className="mb-2">
          <Label htmlFor="email" className="m-2">
            Email Address
          </Label>
          <Input
            id="email"
            type="email"
            placeholder="Enter your email"
            className="mt-2"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
        </div>
        <Button onClick={handleSubmit} className="w-full" disabled={loading}>
          {loading ? "Checking..." : "Verify email"}
        </Button>
      </div>
    </div>
  );
}
