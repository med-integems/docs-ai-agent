import { Github, Mail, Twitter } from "lucide-react";
import Link from "next/link";

export function Footer() {
  return (
    <footer className="w-full bg-primary text-white/90">
      <div className="mx-auto w-full max-w-screen-xl p-4 py-6 lg:py-8">
        <div className="md:flex md:justify-center">
          <div className="mb-6 text-center md:mb-0">
            <Link href="/" className="items-center">
              <div className="flex justify-center text-center flex-row">
                <h1 className="text-2xl text-white font-black md:text-3xl">
                  Docs
                </h1>
                <h1 className="text-2xl text-green-400 font-black md:text-3xl">
                  AI
                </h1>
              </div>
            </Link>
            <p className="mt-2 max-w-xs text-sm text-white/70">
              Your AI-powered document assistant for seamless content generation
              and management.
            </p>
          </div>
       
        </div>
        <hr className="my-6 border-white/10 sm:mx-auto lg:my-8" />
        <div className="sm:flex sm:items-center sm:justify-between">
          <span className="text-sm text-white/70 sm:text-center">
            © {new Date().getFullYear()} DocsAI™. All Rights Reserved. By INTEGEMS ltd
          </span>
          <div className="flex mt-4 space-x-5 sm:justify-center sm:mt-0">
            <Link href="https://integemsgroup.com" className="text-white/70 hover:text-white">
             
              <span className="text-white font-bold">INTEGEMS</span>
            </Link>
            <Link href="#" className="text-white/70 hover:text-white">
              <Github className="w-5 h-5" />
              <span className="sr-only">GitHub</span>
            </Link>
            <Link href="#" className="text-white/70 hover:text-white">
              <Twitter className="w-5 h-5" />
              <span className="sr-only">Twitter</span>
            </Link>
            <Link href="#" className="text-white/70 hover:text-white">
              <Mail className="w-5 h-5" />
              <span className="sr-only">Email</span>
            </Link>
          </div>
        </div>
      </div>
    </footer>
  );
}
