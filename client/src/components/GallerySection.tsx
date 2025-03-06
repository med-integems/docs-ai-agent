import Image from "next/image";
import React from "react";

interface GallerySectionProps {
  imageSrc?: string; // Path to the chat room screenshot
  header?: string; // Header text
  description?: string; // Description text
}

const GallerySection: React.FC<GallerySectionProps> = ({
  imageSrc,
  header,
  description,
}) => {
  return (
    <div className=" flex flex-col items-center justify-center p-6 mb-10">
      {/* Header */}
      {header && (
        <h2 className="text-2xl font-bold text-gray-800 mb-4">{header}</h2>
      )}

      {/* Chat Room Screenshot */}
      <div className="flex flex-col justify-center items-center gap-2 md:flex-row w-full p-5">
        <div className="relative bg-white w-full h-80 rounded-lg overflow-hidden shadow-xl m-4">
          <Image
            src={imageSrc || "/chatroom1.png"}
            alt="Chat Room Screenshot"
            fill
            objectFit="contain"
            className="rounded p-2"
          />
        </div>
        <div className="relative bg-white w-full h-80 rounded-lg overflow-hidden shadow-xl m-4">
          <Image
            src={imageSrc || "/UnReferencedChatRoom.png"}
            alt="Chat Room Screenshot"
            fill
            objectFit="contain"
            className="rounded p-2"
          />
        </div>
      </div>

      {/* Description */}
      {description && (
        <p className="mt-4 text-gray-600 text-center max-w-md">{description}</p>
      )}
    </div>
  );
};

export default GallerySection;
