import {useRef,useState, type PointerEvent, type KeyboardEvent, type MouseEvent} from "react";
import {useLongPress,useMove} from "@react-aria/interactions";
import {mergeProps} from "@react-aria/utils";

export type SwipeSide = "left" | "right" | null;
const quotaWidth=176,accessWidth=108;
const sideOffset=(side:SwipeSide)=>side==="left"?-quotaWidth:side==="right"?accessWidth:0;

export function useUserRowGestures({enabled,side,onChange,onOpen,onMenu,description}: {
  enabled:boolean; side:SwipeSide; onChange:(side:SwipeSide)=>void; onOpen:()=>void; onMenu:()=>void; description:string;
}) {
  const moved=useRef(false),held=useRef(false),cancelled=useRef(false);
  const total=useRef({x:0,y:0}),axis=useRef("pending"),base=useRef(0);
  const [dragOffset,setDragOffset]=useState<number|null>(null);
  const {moveProps}=useMove({
    onMoveStart(){total.current={x:0,y:0};axis.current="pending";base.current=sideOffset(side);},
    onMove({deltaX,deltaY}) {
      if(!enabled||held.current||cancelled.current)return;
      total.current.x+=deltaX;total.current.y+=deltaY;
      const {x,y}=total.current;
      if(axis.current==="pending"&&Math.max(Math.abs(x),Math.abs(y))>7)axis.current=Math.abs(x)>Math.abs(y)?"horizontal":"vertical";
      if(axis.current!=="horizontal")return;
      moved.current=true;setDragOffset(Math.max(-quotaWidth,Math.min(accessWidth,base.current+x)));
    },
    onMoveEnd(){
      setDragOffset(null);
      if(!enabled||held.current||cancelled.current||axis.current!=="horizontal")return;
      if((side==="left"&&total.current.x>54)||(side==="right"&&total.current.x< -54)){onChange(null);return;}
      const x=base.current+total.current.x;
      onChange(x < -54?"left":x > 54?"right":null);
    },
  });
  const {longPressProps}=useLongPress({
    isDisabled:!enabled||dragOffset!==null,threshold:500,accessibilityDescription:description,
    onLongPress(){if(moved.current||cancelled.current||axis.current==="vertical")return;held.current=true;setDragOffset(null);onChange(null);onMenu();},
  });
  const activationProps={
    onPointerDownCapture(){moved.current=false;held.current=false;cancelled.current=false;axis.current="pending";},
    // React Aria's long press emits a synthetic cancellation itself.
    onPointerCancelCapture(e:PointerEvent<HTMLElement>){if(e.nativeEvent.isTrusted){cancelled.current=true;setDragOffset(null);}},
    // Preserve native keyboard activation of buttons nested in the swipe face.
    onKeyDownCapture(e:KeyboardEvent<HTMLElement>){if((e.key==="Enter"||e.key===" ")&&e.target instanceof Element&&e.target.closest("button")){moved.current=false;held.current=false;cancelled.current=false;e.stopPropagation();}},
    onKeyUpCapture(e:KeyboardEvent<HTMLElement>){if((e.key==="Enter"||e.key===" ")&&e.target instanceof Element&&e.target.closest("button"))e.stopPropagation();},
    onClickCapture(e:MouseEvent<HTMLElement>){if(e.detail>0&&(moved.current||held.current||cancelled.current)){e.preventDefault();e.stopPropagation();}},
  };
  const interactionProps=mergeProps(moveProps,longPressProps);
  return {
    gestureProps:enabled?{...interactionProps,onClick(e:MouseEvent<HTMLElement>){interactionProps.onClick?.(e);if(e.target===e.currentTarget&&!moved.current&&!held.current&&!cancelled.current)onOpen();}}:{},
    activationProps:enabled?activationProps:{},
    onTap(){if(!enabled||(!moved.current&&!held.current&&!cancelled.current))onOpen();},
    offset:enabled?(dragOffset??sideOffset(side)):0, dragging:dragOffset!==null,
  };
}
