import { useEffect, useState } from "react";

// Small closing window for non-modal decoration; interaction state changes immediately.
export function usePresence(open: boolean, duration: number) {
  const [present, setPresent] = useState(open);
  useEffect(() => {
    if (open) {
      setPresent(true);
      return;
    }
    const immediate =
      window.matchMedia("(prefers-reduced-motion: reduce)").matches ||
      document.documentElement.dataset.input === "keyboard";
    const timer = setTimeout(() => setPresent(false), immediate ? 0 : duration);
    return () => clearTimeout(timer);
  }, [open, duration]);
  return open || present;
}
