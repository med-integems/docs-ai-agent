import Banner from "@/components/Banner";
import { Footer } from "@/components/Footer";
import GallerySection from "@/components/GallerySection";
import NavBar from "@/components/NavBar";
export default function Home() {
  return (
    <>
      <NavBar />
      <main>
        <Banner />
        <GallerySection />

        <Footer/>
      </main>
    </>
  );
}
