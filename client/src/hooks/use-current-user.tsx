"use client";

import { useEffect, useState } from "react";
import { useCookies } from "react-cookie";
import { jwtDecode } from "jwt-decode";
import { useRouter } from "next/navigation";

type User = {
  userId: string;
  email: string;
  name: string;
  image?: string;
  role: "user" | "admin" | "superadmin";
};

export function useCurrentUser() {
  const [cookies, , removeCookie] = useCookies(["token"]); // Ensure token is stored
  const [user, setUser] = useState<User | null>(null);
  const router = useRouter();

  useEffect(() => {
    if (!cookies.token) {
      setUser(null);
      return;
    }

    try {
      const decoded: User = jwtDecode(cookies.token);
      setUser(decoded);
    } catch{
      // console.error("Invalid token:", error);
      removeCookie("token");
      setUser(null);
    }
  }, [cookies.token, removeCookie]);

  const signOut = () => {
    removeCookie("token", { path: "/" });
    setUser(null);
    router.push("/signin"); // Redirect to SignIn page
  };

  return { user, isAuthenticated: !!user, signOut };
}
