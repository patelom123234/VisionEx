import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import ReactMarkdown from 'react-markdown';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import CircularProgress from '@mui/material/CircularProgress';
import MenuItem from '@mui/material/MenuItem';
import Paper from '@mui/material/Paper';
import Select from '@mui/material/Select';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import ImageWithTitle from './ImageWithTitle';
import { translateToMarkdown } from './grpcweb/client';
import { Language, Model } from './type';
const ImageToMarkdown = ({ state, updateState, }) => {
    const translate = async () => {
        updateState({ isLoading: true });
        try {
            if (!state.image) {
                throw new Error('No image selected');
            }
            const markdown = await translateToMarkdown(state.image.imageBuffer, state.selectedLanguage, state.selectedModel);
            updateState({
                markdown: markdown,
                isLoading: false,
            });
        }
        catch (error) {
            console.error('Error translating image to markdown:', error);
            updateState({ isLoading: false });
        }
    };
    const readFile = (file) => {
        return new Promise((resolve, reject) => {
            const reader = new FileReader();
            reader.onload = () => {
                if (reader.result instanceof ArrayBuffer) {
                    const imageBuffer = new Uint8Array(reader.result);
                    const url = URL.createObjectURL(new Blob([imageBuffer], { type: 'image/png' }));
                    resolve({
                        name: file.name,
                        imageBuffer,
                        url,
                    });
                }
                else {
                    reject(new Error('Failed to read file.'));
                }
            };
            reader.onerror = () => reject(new Error('File reading failed.'));
            reader.readAsArrayBuffer(file);
        });
    };
    const handleImageSelection = async (event) => {
        const files = event.target.files;
        if (!files || files.length === 0)
            return;
        updateState({ isLoading: true });
        try {
            const newImage = await readFile(files[0]);
            updateState({
                image: newImage,
                isLoading: false,
            });
        }
        catch (error) {
            console.error('Error reading file:', error);
            updateState({ isLoading: false });
        }
    };
    return (_jsxs(Stack, { alignItems: 'center', sx: {
            py: 10,
            rowGap: 4,
        }, children: [_jsxs(Button, { variant: 'contained', component: 'label', disabled: state.isLoading, children: ["Select Image", _jsx("input", { type: 'file', accept: 'image/png, image/jpeg, image/jpg', hidden: true, onChange: handleImageSelection })] }), state.image && (_jsxs(Box, { textAlign: 'center', children: [_jsx(Box, { display: 'flex', flexDirection: 'row', flexWrap: 'wrap', justifyContent: 'center', sx: { mt: 4 }, children: _jsx(ImageWithTitle, { src: state.image.url || '', alt: state.image.name, title: state.image.name }) }), _jsxs(Box, { display: 'flex', flexDirection: 'row', gap: 2, justifyContent: 'center', children: [_jsx(Typography, { variant: 'body1', sx: { display: 'flex', alignItems: 'center' }, children: "Result Language:" }), _jsxs(Select, { labelId: 'language-select-label', id: 'language-select', value: state.selectedLanguage, onChange: (e) => updateState({
                                    selectedLanguage: e.target.value,
                                }), children: [_jsx(MenuItem, { value: Language.LANGUAGE_EN_US, children: "English" }), _jsx(MenuItem, { value: Language.LANGUAGE_KO_KR, children: "Korean" }), _jsx(MenuItem, { value: Language.LANGUAGE_JA_JP, children: "Japanese" })] }), _jsx(Typography, { variant: 'body1', sx: { display: 'flex', alignItems: 'center' }, children: "Model:" }), _jsxs(Select, { labelId: 'model-select-label', id: 'model-select', value: state.selectedModel, onChange: (e) => updateState({
                                    selectedModel: e.target.value,
                                }), children: [_jsx(MenuItem, { value: Model.MODEL_GPT4O, children: "Quality (GPT-4o)" }), _jsx(MenuItem, { value: Model.MODEL_GPT4O_MINI, children: "Economical (GPT-4o Mini)" }), _jsx(MenuItem, { value: Model.MODEL_GEMINI_FLASH, children: "Fast (Gemini 1.5 Flash)" })] }), _jsx(Button, { onClick: translate, variant: 'contained', disabled: state.isLoading, sx: { mx: 2 }, children: "CONVERT TO MARKDOWN" })] })] })), state.isLoading && (_jsxs(Box, { textAlign: 'center', sx: { mt: 4 }, children: [_jsx(CircularProgress, {}), _jsx(Typography, { variant: 'body1', sx: { mt: 1 }, children: "Loading image..." })] })), state.markdown && (_jsxs(Box, { sx: { mt: 4, display: 'flex', width: '100%', gap: 2 }, children: [_jsxs(Paper, { elevation: 3, sx: { flex: 1, p: 2, maxWidth: '50%' }, children: [_jsx(Typography, { variant: 'h6', gutterBottom: true, children: "Rendered Markdown" }), _jsx(ReactMarkdown, { children: state.markdown })] }), _jsxs(Paper, { elevation: 3, sx: { flex: 1, p: 2, maxWidth: '50%' }, children: [_jsx(Typography, { variant: 'h6', gutterBottom: true, children: "Raw Markdown" }), _jsx("pre", { style: { whiteSpace: 'pre-wrap', wordBreak: 'break-word' }, children: state.markdown })] })] }))] }));
};
export default ImageToMarkdown;
