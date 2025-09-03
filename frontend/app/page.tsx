"use client";

import { Footer } from "@/components/footer";
import { ShortenerInput } from "@/components/shortener-input";
import { useState, useRef, useEffect } from "react";

export default function Home() {
  const [videoLoaded, setVideoLoaded] = useState(false);
  const videoRef = useRef<HTMLVideoElement>(null);

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;

    // Slow down the video playback
    video.playbackRate = 0.6;

    const handleTimeUpdate = () => {
      // Create seamless loop by resetting to start slightly before the end
      if (video.currentTime >= video.duration - 0.1) {
        video.currentTime = 0;
      }
    };

    video.addEventListener('timeupdate', handleTimeUpdate);
    
    return () => {
      video.removeEventListener('timeupdate', handleTimeUpdate);
    };
  }, []);

  return (
    <main className="min-h-screen w-full relative overflow-hidden">
      {/* Background Image Placeholder */}
      <div 
        className={`absolute inset-0 bg-cover bg-center bg-no-repeat transition-opacity duration-1000 ${
          videoLoaded ? 'opacity-0' : 'opacity-100'
        }`}
        style={{
          backgroundImage: "url('/landscape.png')",
        }}
      />
      
      {/* Background Video */}
      <video
        ref={videoRef}
        autoPlay
        muted
        playsInline
        preload="auto"
        disablePictureInPicture
        disableRemotePlayback
        className={`absolute inset-0 w-full h-full object-cover transition-opacity duration-1000 ${
          videoLoaded ? 'opacity-100' : 'opacity-0'
        }`}
        onLoadedData={() => setVideoLoaded(true)}
        style={{
          willChange: 'transform',
          backfaceVisibility: 'hidden',
          transform: 'scale(1.15)',
        }}
      >
        <source src="/background1.mp4" type="video/mp4" />
      </video>
      
      {/* Overlay */}
      <div className="absolute inset-0 bg-black/30" />
      
      {/* Content */}
      <div className="relative z-10 min-h-screen flex flex-col">
        <div className="flex-1 flex items-center justify-center p-4">
          <ShortenerInput />
        </div>
        <Footer />
      </div>
    </main>
  );
}
