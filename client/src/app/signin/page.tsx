"use client";

import React, { useState } from "react";
import { useRouter } from "next/navigation";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { useToast } from "@/hooks/use-toast";
import { ArrowLeft, Loader2Icon, LogIn } from "lucide-react";
import { useCookies } from "react-cookie";
import Link from "next/link";
import Image from "next/image";

export default function SignInPage() {
  const [formData, setFormData] = useState({ email: "", password: "" });
  const [loading, setLoading] = useState(false);
  const router = useRouter();
  const { toast } = useToast();
  const [,setCookie] = useCookies(["token"]);

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    setFormData((prev) => ({ ...prev, [name]: value }));
  };

  const handleSubmit = async () => {
    setLoading(true);

    try {
      const response = await fetch("http://127.0.0.1:5000/signin", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(formData),
      });

      const data = await response.json();

      if (!response.ok) {
        throw new Error(data.message || "Invalid email or password.");
      }
      setCookie("token", data.token);
      router.push("/");
    } catch  {
      // console.error(error);
      toast({
        description: (
          <p className="text-red-500">{"Invalid email or password."}</p>
        ),
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex min-h-screen  justify-center bg-gray-100 pt-[5vh]">
      <div className="w-full max-w-md p-8 h-fit bg-white shadow-lg rounded-lg">
        <Button
          variant="ghost"
          size="icon"
          className="text-primary font-bold"
          onClick={() => router.push("/")}
        >
          <ArrowLeft strokeWidth={3} />
        </Button>
        <div className="relative w-full m:max-w-[15vw] md:w-[15vw] mx-auto aspect-square">
          <Image
            alt="Login image"
            fill
            className="absolute"
            src={"/login.gif"}
          />
        </div>
        <div className="space-y-4">
          {/* Email Field */}
          <div>
            <Label htmlFor="email">Email</Label>
            <Input
              id="email"
              name="email"
              type="email"
              placeholder="Enter your email"
              className="mt-1"
              value={formData.email}
              onChange={handleInputChange}
              required
            />
          </div>

          {/* Password Field */}
          <div className="mb-2">
            <Label htmlFor="password">Password</Label>
            <Input
              id="password"
              name="password"
              type="password"
              className="mt-1"
              placeholder="Enter your password"
              value={formData.password}
              onChange={handleInputChange}
              required
            />
          </div>

          {/* Submit Button */}
          <Button
            className="w-full mt-4"
            onClick={handleSubmit}
            disabled={loading}
          >
            {loading && <Loader2Icon className="animate-spin mr-2" />}
            <LogIn className="mr-2" />
            Sign In
          </Button>
          <Button variant="link" asChild>
            <Link href="/verify-email">Forget password</Link>
          </Button>
        </div>
      </div>
    </div>
  );
}
