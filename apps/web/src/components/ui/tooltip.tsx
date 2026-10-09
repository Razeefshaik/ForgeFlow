import { Slot } from "@radix-ui/react-slot";
import { createPortal } from "react-dom";
import {
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type ReactElement,
} from "react";
import { usePresence } from "./presence";

// Adjacent toolbar hints skip the initial delay. No animation dependency needed.
let lastDismissed = 0;

export function Tooltip({
  content,
  children,
  disabled = false,
  side = "top",
}: {
  content: string;
  children: ReactElement;
  disabled?: boolean;
  side?: "top" | "right";
}) {
  const id = useId();
  const trigger = useRef<HTMLElement>(null);
  const tip = useRef<HTMLDivElement>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [open, setOpen] = useState(false);
  const present = usePresence(open, 120);
  const [instant, setInstant] = useState(false);
  const [position, setPosition] = useState<{
    top: number;
    left: number;
    side: string;
  } | null>(null);
  const clearTimer = () => {
    if (timer.current) clearTimeout(timer.current);
    timer.current = null;
  };
  const close = () => {
    clearTimer();
    setOpen(false);
    if (open) lastDismissed = Date.now();
  };
  const show = (keyboard = false) => {
    if (disabled) return;
    clearTimer();
    const immediate = keyboard || Date.now() - lastDismissed < 700;
    setInstant(immediate);
    timer.current = setTimeout(() => setOpen(true), immediate ? 0 : 350);
  };
  const hideSoon = () => {
    clearTimer();
    timer.current = setTimeout(close, 100);
  };
  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
    },
    [],
  );
  useEffect(() => {
    if (disabled) {
      if (timer.current) clearTimeout(timer.current);
      setOpen(false);
      setPosition(null);
    }
  }, [disabled]);
  useLayoutEffect(() => {
    if (!open) return;
    const place = () => {
      if (!trigger.current || !tip.current) return;
      const anchor = trigger.current.getBoundingClientRect();
      const bounds = tip.current.getBoundingClientRect();
      const above = anchor.top >= bounds.height + 12;
      const beside =
        side === "right" && anchor.right + bounds.width + 16 <= innerWidth;
      setPosition({
        left: Math.max(
          8,
          Math.min(
            beside
              ? anchor.right + 8
              : anchor.left + anchor.width / 2 - bounds.width / 2,
            innerWidth - bounds.width - 8,
          ),
        ),
        top: Math.max(
          8,
          Math.min(
            beside
              ? anchor.top + anchor.height / 2 - bounds.height / 2
              : above
                ? anchor.top - bounds.height - 8
                : anchor.bottom + 8,
            innerHeight - bounds.height - 8,
          ),
        ),
        side: beside ? "right" : above ? "top" : "bottom",
      });
    };
    const escape = (event: KeyboardEvent) => {
      if (event.key === "Escape") close();
    };
    place();
    window.addEventListener("resize", place);
    window.addEventListener("scroll", place, true);
    document.addEventListener("keydown", escape);
    return () => {
      window.removeEventListener("resize", place);
      window.removeEventListener("scroll", place, true);
      document.removeEventListener("keydown", escape);
    };
  }, [open, content, side]);
  return (
    <>
      <Slot
        ref={trigger}
        aria-describedby={open ? id : undefined}
        onPointerEnter={(event) => {
          if (
            event.pointerType !== "touch" &&
            window.matchMedia("(hover: hover) and (pointer: fine)").matches
          )
            show();
        }}
        onPointerLeave={hideSoon}
        onFocus={(event) => {
          if (event.target.matches(":focus-visible")) show(true);
        }}
        onBlur={close}
        onClick={close}
      >
        {children}
      </Slot>
      {present &&
        !disabled &&
        createPortal(
          <div
            ref={tip}
            id={id}
            role="tooltip"
            className="tooltip"
            data-instant={instant}
            data-state={open ? "open" : "closed"}
            aria-hidden={!open}
            data-side={position?.side}
            style={{
              top: position?.top ?? 0,
              left: position?.left ?? 0,
              visibility: position ? "visible" : "hidden",
            }}
            onPointerEnter={clearTimer}
            onPointerLeave={hideSoon}
          >
            {content}
          </div>,
          document.body,
        )}
    </>
  );
}
