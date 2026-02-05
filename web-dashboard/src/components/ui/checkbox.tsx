import * as React from "react";
import { cn } from "@/lib/utils";
import { Check, Minus } from "lucide-react";

interface CheckboxProps extends Omit<React.ComponentProps<"input">, "type"> {
  indeterminate?: boolean;
}

const Checkbox = React.forwardRef<HTMLInputElement, CheckboxProps>(
  ({ className, indeterminate, checked, ...props }, ref) => {
    const innerRef = React.useRef<HTMLInputElement>(null);

    React.useImperativeHandle(ref, () => innerRef.current!);

    React.useEffect(() => {
      if (innerRef.current) {
        innerRef.current.indeterminate = indeterminate ?? false;
      }
    }, [indeterminate]);

    return (
      <div className="relative inline-flex items-center">
        <input
          type="checkbox"
          ref={innerRef}
          checked={checked}
          className={cn(
            "peer h-4 w-4 shrink-0 rounded border border-input bg-background shadow-sm",
            "focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring",
            "disabled:cursor-not-allowed disabled:opacity-50",
            "appearance-none cursor-pointer",
            "checked:bg-primary checked:border-primary",
            "[&:indeterminate]:bg-primary [&:indeterminate]:border-primary",
            className
          )}
          {...props}
        />
        <Check
          className={cn(
            "absolute left-0.5 top-0.5 h-3 w-3 text-primary-foreground pointer-events-none",
            "opacity-0 peer-checked:opacity-100",
            indeterminate && "opacity-0"
          )}
        />
        <Minus
          className={cn(
            "absolute left-0.5 top-0.5 h-3 w-3 text-primary-foreground pointer-events-none",
            "opacity-0",
            indeterminate && "opacity-100"
          )}
        />
      </div>
    );
  }
);
Checkbox.displayName = "Checkbox";

export { Checkbox };

