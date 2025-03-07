"use client";

import React, { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useToast } from "@/hooks/use-toast";
import { Loader2Icon, Plus } from "lucide-react";
import { useRouter } from "next/navigation";

export default function AddUserModal() {
  const [formData, setFormData] = useState({
    name: "",
    email: "",
    password: "",
    confirmPassword: "",
  });
  const [loading, setLoading] = useState(false);
  const router = useRouter();
  const { toast } = useToast();

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    setFormData((prev) => ({ ...prev, [name]: value }));
  };

  const handleSubmit = async () => {
    setLoading(true);
    if (formData.password !== formData.confirmPassword) {
      toast({
        description: <p className="text-red-500">Passwords do not match.</p>,
      });
      setLoading(false);
      return;
    }

    try {
      const response = await fetch("http://127.0.0.1:5000/signup", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          name: formData.name,
          email: formData.email,
          password: formData.password,
        }),
      });

      const data = await response.json();

      if (!response.ok) {
        throw new Error(data.error || "Failed to sign up user.");
      }

      toast({
        description: (
          <p className="text-green-500">
            {data.message || "User registered successfully"}
          </p>
        ),
      });

      // Optionally reset the form and/or refresh the page
      setFormData({
        name: "",
        email: "",
        password: "",
        confirmPassword: "",
      });
      router.refresh();
    } catch {
      // console.error(error);
      toast({
        description: (
          <p className="text-red-500">
            Couldn&apos;t sign up user.
          </p>
        ),
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button variant="default" size="sm">
          <Plus className="mr-2" /> Add User
        </Button>
      </DialogTrigger>
      <DialogContent className="overflow-x-hidden">
        <DialogHeader>
          <h2 className="text-lg font-semibold">Add New User</h2>
        </DialogHeader>

        <div className="space-y-4">
          {/* Name Field */}
          <div>
            <Label htmlFor="name" className="mb-2">
              Name
            </Label>
            <Input
              id="name"
              name="name"
              placeholder="Enter name"
              value={formData.name}
              onChange={handleInputChange}
              className="w-full"
              required
            />
          </div>
          {/* Email Field */}
          <div>
            <Label htmlFor="email" className="mb-2">
              Email
            </Label>
            <Input
              id="email"
              name="email"
              type="email"
              placeholder="Enter email"
              value={formData.email}
              onChange={handleInputChange}
              className="w-full"
              required
            />
          </div>
          {/* Password Field */}
          <div>
            <Label htmlFor="password" className="mb-2">
              Password
            </Label>
            <Input
              id="password"
              name="password"
              type="password"
              placeholder="Enter password"
              value={formData.password}
              onChange={handleInputChange}
              className="w-full"
              required
            />
          </div>
          {/* Confirm Password Field */}
          <div>
            <Label htmlFor="confirmPassword" className="mb-2">
              Confirm Password
            </Label>
            <Input
              id="confirmPassword"
              name="confirmPassword"
              type="password"
              placeholder="Confirm password"
              value={formData.confirmPassword}
              onChange={handleInputChange}
              className="w-full"
              required
            />
          </div>
        </div>

        <DialogFooter>
          <Button size="sm" onClick={handleSubmit} disabled={loading}>
            {loading && <Loader2Icon className="mr-2 animate-spin" />}
            Continue
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
