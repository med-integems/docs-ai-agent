"use client";
import { Nunito_Sans } from "next/font/google";
import "./globals.css";
import { Toaster } from "@/components/ui/toaster";
import { CookiesProvider } from "react-cookie";
import { Footer } from "@/components/Footer";

const nunitoSans = Nunito_Sans({
  variable: "--font-nunito-sans",
  subsets: ["latin"],
});

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <CookiesProvider>
      <html lang="en">
        <body className={`${nunitoSans.variable} antialiased`}>
          {children}
          {/* <Footer /> */}
          <Toaster />
        </body>
      </html>
    </CookiesProvider>
  );
}
