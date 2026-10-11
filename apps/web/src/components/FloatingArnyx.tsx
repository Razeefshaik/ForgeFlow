import { useEffect, useRef, useState, type CSSProperties } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { Link } from "react-router-dom";
import { ArrowUpRight, X } from "./icons";
import Arnyx from "./Arnyx";

type Position = { x: number; y: number };
const clamp = (p: Position) => ({
  x: Math.max(12, Math.min(p.x, window.innerWidth - 88)),
  y: Math.max(12, Math.min(p.y, window.innerHeight - 100)),
});
export default function FloatingArnyx({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const [position, setPosition] = useState<Position>(() => {
    try {
      const saved = JSON.parse(
        localStorage.getItem("forgeflow-arnyx-position") || "null",
      );
      if (saved && Number.isFinite(saved.x) && Number.isFinite(saved.y))
        return clamp(saved);
    } catch {
      /* Ignore malformed local preferences. */
    }
    return clamp({ x: window.innerWidth - 108, y: window.innerHeight - 124 });
  });
  const [dragging, setDragging] = useState(false);
  const drag = useRef<{
    x: number;
    y: number;
    origin: Position;
    moved: boolean;
  } | null>(null);
  const suppressClick = useRef(false);
  const opener = useRef<HTMLElement | null>(null);
  const launcher = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    const resize = () => setPosition((p) => clamp(p));
    window.addEventListener("resize", resize);
    return () => window.removeEventListener("resize", resize);
  }, []);
  const height = Math.min(650, window.innerHeight - 128);
  const panelStyle: CSSProperties = {
    left: Math.max(12, Math.min(position.x - 320, window.innerWidth - 432)),
    top: Math.max(
      12,
      Math.min(
        position.y > window.innerHeight / 2
          ? position.y - height - 12
          : position.y + 88,
        window.innerHeight - height - 12,
      ),
    ),
    height,
  };
  return (
    <>
      <button
        ref={launcher}
        className="arnyx-companion"
        data-dragging={dragging}
        style={{
          left: `min(${position.x}px, calc(100vw - 88px))`,
          top: `min(${position.y}px, calc(100dvh - 100px))`,
        }}
        aria-label="Chat with Arnyx"
        aria-haspopup="dialog"
        aria-expanded={open}
        aria-describedby="arnyx-drag-hint"
        onPointerDown={(e) => {
          if (e.button !== 0) return;
          drag.current = {
            x: e.clientX,
            y: e.clientY,
            origin: {
              x: e.currentTarget.getBoundingClientRect().x,
              y: e.currentTarget.getBoundingClientRect().y,
            },
            moved: false,
          };
          e.currentTarget.setPointerCapture(e.pointerId);
        }}
        onPointerMove={(e) => {
          const d = drag.current;
          if (!d) return;
          const dx = e.clientX - d.x,
            dy = e.clientY - d.y;
          if (Math.hypot(dx, dy) > 6) d.moved = true;
          if (d.moved) {
            setDragging(true);
            setPosition(clamp({ x: d.origin.x + dx, y: d.origin.y + dy }));
          }
        }}
        onPointerUp={(e) => {
          if (drag.current) {
            suppressClick.current = drag.current.moved;
            drag.current = null;
            setDragging(false);
            localStorage.setItem(
              "forgeflow-arnyx-position",
              JSON.stringify(position),
            );
          }
          if (e.currentTarget.hasPointerCapture(e.pointerId))
            e.currentTarget.releasePointerCapture(e.pointerId);
        }}
        onPointerCancel={() => {
          drag.current = null;
          setDragging(false);
          suppressClick.current = true;
        }}
        onKeyDown={(e) => {
          const delta: Record<string, Position> = {
            ArrowLeft: { x: -20, y: 0 },
            ArrowRight: { x: 20, y: 0 },
            ArrowUp: { x: 0, y: -20 },
            ArrowDown: { x: 0, y: 20 },
          };
          if (delta[e.key]) {
            e.preventDefault();
            const next = clamp({
              x: position.x + delta[e.key].x,
              y: position.y + delta[e.key].y,
            });
            setPosition(next);
            localStorage.setItem(
              "forgeflow-arnyx-position",
              JSON.stringify(next),
            );
          }
        }}
        onClick={() => {
          if (suppressClick.current) {
            suppressClick.current = false;
            return;
          }
          onOpenChange(!open);
        }}
      >
        <span className="arnyx-orbit" />
        <img draggable={false} src="/images/arnyx-avatar.svg" alt="" />
        <span className="arnyx-companion-name">Arnyx</span>
        <span className="arnyx-greeting" aria-hidden="true">
          Hey, I’m here ✦
        </span>
      </button>
      <span id="arnyx-drag-hint" className="sr-only">
        Drag to move. Arrow keys reposition Arnyx. Enter opens chat.
      </span>
      <Dialog.Root open={open} onOpenChange={onOpenChange} modal={false}>
        <Dialog.Portal>
          <Dialog.Content
            className="arnyx-panel"
            style={panelStyle}
            onOpenAutoFocus={(e) => {
              opener.current =
                document.activeElement instanceof HTMLElement
                  ? document.activeElement
                  : launcher.current;
              e.preventDefault();
              (e.target as HTMLElement)
                .querySelector<HTMLInputElement>("[data-autofocus]")
                ?.focus();
            }}
            onCloseAutoFocus={(e) => {
              e.preventDefault();
              (opener.current?.isConnected
                ? opener.current
                : launcher.current
              )?.focus();
            }}
            onInteractOutside={(e) => e.preventDefault()}
          >
            <header className="arnyx-panel-header">
              <div className="arnyx-header-avatar">
                <img src="/images/arnyx-avatar.svg" alt="" />
              </div>
              <div>
                <Dialog.Title>Arnyx</Dialog.Title>
                <Dialog.Description>
                  Your project, a little closer.
                </Dialog.Description>
              </div>
              <Link
                className="icon-button"
                aria-label="Expand Arnyx"
                to="/arnyx"
                onClick={() => onOpenChange(false)}
              >
                <ArrowUpRight size={18} />
              </Link>
              <Dialog.Close className="icon-button" aria-label="Close dialog">
                <X size={18} />
              </Dialog.Close>
            </header>
            <Arnyx />
          </Dialog.Content>
        </Dialog.Portal>
      </Dialog.Root>
    </>
  );
}
