import * as DialogPrimitive from "@radix-ui/react-dialog";
import { X } from "../icons";
import { useRef, type ReactNode } from "react";
export function Dialog({
  open,
  onOpenChange,
  title,
  description,
  children,
  wide = false,
  drawer = false,
  instant = false,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  title: string;
  description: string;
  children: ReactNode;
  wide?: boolean;
  drawer?: boolean;
  instant?: boolean;
}) {
  const opener = useRef<HTMLElement | null>(null);
  return (
    <DialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
      <DialogPrimitive.Portal>
        <DialogPrimitive.Overlay
          className="dialog-overlay"
          data-instant={instant}
        />
        <DialogPrimitive.Content
          data-instant={instant}
          onOpenAutoFocus={(event) => {
            opener.current =
              document.activeElement instanceof HTMLElement
                ? document.activeElement
                : null;
            const preferred = (
              event.target as HTMLElement | null
            )?.querySelector<HTMLElement>("[data-autofocus]");
            if (preferred) {
              event.preventDefault();
              preferred.focus();
            }
          }}
          onCloseAutoFocus={(event) => {
            if (opener.current?.isConnected) {
              event.preventDefault();
              opener.current.focus();
            }
          }}
          className={
            "dialog-content" +
            (wide ? " dialog-wide" : "") +
            (drawer ? " dialog-drawer" : "")
          }
        >
          <div className="dialog-heading">
            <div>
              <DialogPrimitive.Title>{title}</DialogPrimitive.Title>
              <DialogPrimitive.Description>
                {description}
              </DialogPrimitive.Description>
            </div>
            <DialogPrimitive.Close
              className="icon-button"
              aria-label="Close dialog"
            >
              <X size={18} />
            </DialogPrimitive.Close>
          </div>
          <div className="dialog-body">{children}</div>
        </DialogPrimitive.Content>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  );
}
