import { TabState } from './type';
declare const ImageToText: ({ state, updateState, }: {
    state: TabState["text"];
    updateState: (newState: Partial<TabState["text"]>) => void;
}) => import("react/jsx-runtime").JSX.Element;
export default ImageToText;
