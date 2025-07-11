import { TabState } from './type';
declare const ImageToImage: ({ state, updateState, }: {
    state: TabState["image"];
    updateState: (newState: Partial<TabState["image"]>) => void;
}) => import("react/jsx-runtime").JSX.Element;
export default ImageToImage;
