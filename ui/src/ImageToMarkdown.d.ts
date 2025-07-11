import { TabState } from './type';
declare const ImageToMarkdown: ({ state, updateState, }: {
    state: TabState["markdown"];
    updateState: (newState: Partial<TabState["markdown"]>) => void;
}) => import("react/jsx-runtime").JSX.Element;
export default ImageToMarkdown;
