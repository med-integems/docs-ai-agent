"use client";

import { useState, useMemo, useId, useEffect } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { useToast } from "@/hooks/use-toast";
import {
  CheckIcon,
  EyeIcon,
  EyeOffIcon,
  XIcon,
  Loader2Icon,
  ArrowLeftCircle,
  ArrowLeft,
} from "lucide-react";

export default function NewPasswordPage() {
  const id = useId();
  const { toast } = useToast();
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [isVisible, setIsVisible] = useState(false);
  const [loading, setLoading] = useState(false);
  const searchParams = useSearchParams();
  const router = useRouter();

  // Extract `userId` from query parameters
  const userId = searchParams.get("userId");

  useEffect(() => {
    if (!userId) {
      toast({
        description: <p className="text-red-500">Invalid or missing user ID</p>,
      });
      router.push("/"); // Redirect if userId is missing
    }
  }, [userId, router, toast]);

  const toggleVisibility = () => setIsVisible((prev) => !prev);

  const checkStrength = (pass: string) =>
    [
      { regex: /.{8,}/, text: "At least 8 characters" },
      { regex: /[0-9]/, text: "At least 1 number" },
      { regex: /[a-z]/, text: "At least 1 lowercase letter" },
      { regex: /[A-Z]/, text: "At least 1 uppercase letter" },
    ].map((req) => ({ met: req.regex.test(pass), text: req.text }));

  const strength = checkStrength(password);
  const strengthScore = useMemo(
    () => strength.filter((req) => req.met).length,
    [strength],
  );

  const getStrengthColor = (score: number) => {
    if (score === 0) return "bg-border";
    if (score <= 1) return "bg-red-500";
    if (score <= 2) return "bg-orange-500";
    if (score === 3) return "bg-amber-500";
    return "bg-emerald-500";
  };

  const getStrengthText = (score: number) => {
    if (score === 0) return "Enter a password";
    if (score <= 2) return "Weak password";
    if (score === 3) return "Medium password";
    return "Strong password";
  };

  const handleSubmit = async () => {
    setLoading(true);

    if (!password || !userId) {
      toast({
        description: (
          <p className="text-red-500">Password and User ID are required</p>
        ),
      });
      setLoading(false);
      return;
    }

    if (password !== confirmPassword) {
      toast({
        description: <p className="text-red-500">Passwords do not match</p>,
      });
      setLoading(false);
      return;
    }

    try {
      const response = await fetch("http://127.0.0.1:5000/new-password", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ userId, newPassword: password }),
      });

      const data = await response.json();
      if (!response.ok)
        throw new Error(data.error || "Failed to update password");

      toast({
        description: (
          <p className="text-green-500">Password updated successfully!</p>
        ),
      });
      router.push("/signin");
    } catch (error: any) {
      toast({ description: <p className="text-red-500">{error.message}</p> });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="max-w-md mt-[20vh] mx-auto p-6 bg-white rounded-lg shadow-md">
      <Button
        variant="ghost"
        size="icon"
        className="text-primary font-bold"
        onClick={() => router.back()}
      >
        <ArrowLeft strokeWidth={3} />
      </Button>
      <h2 className="text-lg font-semibold text-center mb-4">
        Set New Password
      </h2>
      {/* <Label htmlFor={id} className="m-2">
        Enter your new password
      </Label> */}
      <div className="relative mt-2">
        <Input
          id={id}
          className="pe-9 mt-2"
          placeholder="New Password"
          type={isVisible ? "text" : "password"}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
        <button
          className="absolute inset-y-0 end-0 flex h-full w-9 items-center justify-center"
          onClick={toggleVisibility}
          aria-label="Toggle Password"
        >
          {isVisible ? <EyeOffIcon size={16} /> : <EyeIcon size={16} />}
        </button>
      </div>

      {/* <Label htmlFor={`confirm-${id}`} className="m-2">
        Confirm password
      </Label> */}
      <div className="relative mt-2">
        <Input
          id={`confirm-${id}`}
          className="pe-9 mt-2"
          placeholder="Confirm New Password"
          type={isVisible ? "text" : "password"}
          value={confirmPassword}
          onChange={(e) => setConfirmPassword(e.target.value)}
        />
        <button
          className="absolute inset-y-0 end-0 flex h-full w-9 items-center justify-center"
          onClick={toggleVisibility}
          aria-label="Toggle Password"
        >
          {isVisible ? <EyeOffIcon size={16} /> : <EyeIcon size={16} />}
        </button>
      </div>
      {confirmPassword && confirmPassword !== password && (
        <p className="text-red-500 text-xs mt-1">Passwords do not match</p>
      )}

      <Button
        className="w-full mt-4"
        onClick={handleSubmit}
        disabled={strengthScore < 4 || loading || password !== confirmPassword}
      >
        {loading && <Loader2Icon className="animate-spin mr-2" />} Update
        Password
      </Button>

      {/* Password strength indicator */}
      <div
        className="bg-border mt-3 mb-4 h-1 w-full overflow-hidden rounded-full"
        role="progressbar"
        aria-valuenow={strengthScore}
        aria-valuemin={0}
        aria-valuemax={4}
        aria-label="Password strength"
      >
        <div
          className={`h-full ${getStrengthColor(strengthScore)} transition-all duration-500 ease-out`}
          style={{ width: `${(strengthScore / 4) * 100}%` }}
        ></div>
      </div>

      {/* Password strength description */}
      <p
        id={`${id}-description`}
        className="text-foreground mb-2 text-sm font-medium"
      >
        {getStrengthText(strengthScore)}. Must contain:
      </p>

      {/* Password requirements list */}
      <ul className="space-y-1.5" aria-label="Password requirements">
        {strength.map((req, index) => (
          <li key={index} className="flex items-center gap-2">
            {req.met ? (
              <CheckIcon
                size={16}
                className="text-emerald-500"
                aria-hidden="true"
              />
            ) : (
              <XIcon
                size={16}
                className="text-muted-foreground/80"
                aria-hidden="true"
              />
            )}
            <span
              className={`text-xs ${req.met ? "text-emerald-600" : "text-muted-foreground"}`}
            >
              {req.text}
              <span className="sr-only">
                {req.met ? " - Requirement met" : " - Requirement not met"}
              </span>
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}
