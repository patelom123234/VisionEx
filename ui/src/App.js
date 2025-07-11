import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { useState } from 'react';
import Stack from '@mui/material/Stack';
import Tab from '@mui/material/Tab';
import Tabs from '@mui/material/Tabs';
import ImageToImage from './ImageToImage';
import ImageToMarkdown from './ImageToMarkdown';
import ImageToText from './ImageToText';
import SignIn from './Signin';
import { VISIONEX_TOKEN_KEY } from './grpcweb/client';
import { Language, Model } from './type';
const App = () => {
    const [currentTab, setCurrentTab] = useState(0);
    const [isAuthenticated, setIsAuthenticated] = useState(!!localStorage.getItem(VISIONEX_TOKEN_KEY));
    const [tabState, setTabState] = useState({
        markdown: {
            image: null,
            markdown: null,
            selectedLanguage: Language.LANGUAGE_EN_US,
            selectedModel: Model.MODEL_GPT4O,
            isLoading: false,
        },
        image: {
            image: null,
            translatedImage: null,
            selectedLanguage: Language.LANGUAGE_EN_US,
            isLoading: false,
        },
        text: {
            image: null,
            result: null,
            selectedLanguage: Language.LANGUAGE_EN_US,
            isLoading: false,
        },
    });
    const updateTabState = (tab, newState) => {
        setTabState((prev) => ({
            ...prev,
            [tab]: { ...prev[tab], ...newState },
        }));
    };
    return !isAuthenticated ? (_jsx(SignIn, { setAuthenticated: setIsAuthenticated })) : (_jsxs(Stack, { children: [_jsxs(Tabs, { value: currentTab, onChange: (_, newValue) => setCurrentTab(newValue), children: [_jsx(Tab, { label: 'Markdown' }), _jsx(Tab, { label: 'Image' }), _jsx(Tab, { label: 'Text' })] }), currentTab === 0 && (_jsx(ImageToMarkdown, { state: tabState.markdown, updateState: (newState) => updateTabState('markdown', newState) })), currentTab === 1 && (_jsx(ImageToImage, { state: tabState.image, updateState: (newState) => updateTabState('image', newState) })), currentTab === 2 && (_jsx(ImageToText, { state: tabState.text, updateState: (newState) => updateTabState('text', newState) }))] }));
};
export default App;
