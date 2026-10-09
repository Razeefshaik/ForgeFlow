import { useEffect, type RefObject } from "react";

// Progressive decoration only: content remains visible if observers or motion fail.
const cards = ".surface, .metric, .contribution-summary > div, .agent-summary > div, .usage-summary > div, .contribution-grid .contribution-row, .opportunity-inspector, .stage-rail, .execution-commandbar, .live-execution";

export function useVisualEffects(scope: RefObject<HTMLDivElement | null>, route: string, paused: boolean) {
  useEffect(() => {
    const root = scope.current;
    if (!root) return;
    const reduced = matchMedia("(prefers-reduced-motion: reduce)");
    const finePointer = matchMedia("(hover: hover) and (pointer: fine)");
    const seen = new WeakSet<Element>();
    const animations = new Set<Animation>();
    const canAnimate = () => !paused && !reduced.matches && document.documentElement.dataset.input !== "keyboard";
    const finish = () => { animations.forEach(animation => animation.finish()); animations.clear(); };
    const reveal = new IntersectionObserver(entries => {
      let stagger = 0;
      for (const entry of entries) {
        if (!entry.isIntersecting) continue;
        const element = entry.target as HTMLElement;
        reveal.unobserve(element);
        element.dataset.revealed = "true";
        if (!canAnimate()) continue;
        const animation = element.animate([
          { opacity: 0.25, transform: "translateY(22px) scale(0.985)" },
          { opacity: 1, transform: "translateY(0) scale(1)" },
        ], { duration: 650, delay: Math.min(stagger++ * 55, 220), easing: "cubic-bezier(.16,1,.3,1)" });
        animations.add(animation);
        animation.onfinish = animation.oncancel = () => animations.delete(animation);
      }
    }, { threshold: 0.06 });
    const artwork = new IntersectionObserver(entries => {
      entries.forEach(entry => entry.target.toggleAttribute("data-in-view", entry.isIntersecting));
    });
    const scan = () => {
      root.querySelectorAll<HTMLElement>(cards).forEach(element => {
        element.classList.add("glow-card");
        if (seen.has(element)) return;
        seen.add(element);
        // Nested evidence and table updates stay still while being read.
        if (element.dataset.revealed !== "true" && !element.parentElement?.closest(".glow-card")) reveal.observe(element);
      });
      root.querySelectorAll(".hero-art").forEach(element => artwork.observe(element));
    };
    let scanFrame = 0;
    const mutations = new MutationObserver(() => {
      cancelAnimationFrame(scanFrame);
      scanFrame = requestAnimationFrame(scan);
    });
    mutations.observe(root, { subtree: true, childList: true });
    scan();
    let pointerFrame = 0;
    const move = (event: PointerEvent) => {
      if (!finePointer.matches || event.pointerType === "touch") return;
      const target = event.target instanceof Element ? event.target : null;
      const card = target?.closest<HTMLElement>(".glow-card");
      const art = target?.closest<HTMLElement>(".hero-art");
      cancelAnimationFrame(pointerFrame);
      pointerFrame = requestAnimationFrame(() => {
        if (card) {
          const bounds = card.getBoundingClientRect();
          card.style.setProperty("--pointer-x", `${event.clientX - bounds.left}px`);
          card.style.setProperty("--pointer-y", `${event.clientY - bounds.top}px`);
        }
        if (art && canAnimate()) {
          const bounds = art.getBoundingClientRect();
          art.style.setProperty("--art-x", `${(event.clientX - bounds.left - bounds.width / 2) / 28}deg`);
          art.style.setProperty("--art-y", `${-(event.clientY - bounds.top - bounds.height / 2) / 28}deg`);
        }
      });
    };
    const resetArt = (event: PointerEvent) => {
      const art = event.target instanceof Element ? event.target.closest<HTMLElement>(".hero-art") : null;
      if (art && !(event.relatedTarget instanceof Node && art.contains(event.relatedTarget))) art.removeAttribute("style");
    };
    const visibility = () => { document.documentElement.toggleAttribute("data-hidden", document.hidden); };
    root.addEventListener("pointermove", move, { passive: true });
    root.addEventListener("pointerout", resetArt);
    document.addEventListener("keydown", finish, true);
    document.addEventListener("visibilitychange", visibility);
    reduced.addEventListener("change", finish);
    return () => {
      finish(); reveal.disconnect(); artwork.disconnect(); mutations.disconnect();
      cancelAnimationFrame(scanFrame); cancelAnimationFrame(pointerFrame);
      root.removeEventListener("pointermove", move);
      root.removeEventListener("pointerout", resetArt);
      document.removeEventListener("keydown", finish, true);
      document.removeEventListener("visibilitychange", visibility);
      reduced.removeEventListener("change", finish);
    };
  }, [scope, route, paused]);
}
