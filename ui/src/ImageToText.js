import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import CircularProgress from '@mui/material/CircularProgress';
import MenuItem from '@mui/material/MenuItem';
import Select from '@mui/material/Select';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import { translateTextFromImage } from './grpcweb/client';
import { Language } from './type';
const ImageToText = ({ state, updateState, }) => {
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
                        id: Math.random().toString(36).substring(2, 15),
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
        try {
            const newImage = await readFile(files[0]);
            updateState({
                image: newImage,
                result: null,
            });
        }
        catch (error) {
            console.error('Error reading file:', error);
        }
    };
    const handleProcessImage = async () => {
        if (!state.image) {
            alert('Please select an image first');
            return;
        }
        updateState({ isLoading: true });
        try {
            const result = await processImage(state.image);
            updateState({
                result,
                isLoading: false,
            });
        }
        catch (error) {
            console.error('Error processing image:', error);
            updateState({ isLoading: false });
        }
    };
    const processImage = async (image) => {
        try {
            const imageData = await translateTextFromImage(image.imageBuffer, state.selectedLanguage);
            return {
                id: image.id,
                originalImage: image,
                translatedImage: imageData.uriImage,
                sentences: imageData.sentences,
                isLoading: false,
            };
        }
        catch (error) {
            console.error('Error processing image:', error);
            if (error instanceof Error && error.message === 'ResourceExhausted') {
                alert('Too many requests. Please try again later.');
            }
            return {
                id: image.id,
                originalImage: image,
                translatedImage: image.url,
                sentences: [{ text: 'Error processing image' }],
                isLoading: false,
            };
        }
    };
    const renderPairedTexts = (originalTexts, translatedTexts) => {
        const maxLength = Math.max(originalTexts.length, translatedTexts.length);
        const pairs = Array.from({ length: maxLength }, (_, i) => ({
            original: originalTexts[i] || '',
            translated: translatedTexts[i] || '',
        }));
        return (_jsxs(Box, { sx: { width: '100%' }, children: [_jsxs(Box, { sx: {
                        display: 'grid',
                        gridTemplateColumns: '1fr 1fr',
                        gap: 2,
                        bgcolor: 'background.paper',
                        borderRadius: 1,
                        p: 2,
                        borderBottom: '2px solid #eee',
                    }, children: [_jsx(Typography, { variant: 'h6', sx: { fontWeight: 'bold' }, children: "Original Text" }), _jsx(Typography, { variant: 'h6', sx: { fontWeight: 'bold' }, children: "Translated Text" })] }), pairs.map((pair, index) => (_jsxs(Box, { sx: {
                        display: 'grid',
                        gridTemplateColumns: '1fr 1fr',
                        gap: 2,
                        bgcolor: index % 2 === 0 ? 'rgba(0, 0, 0, 0.02)' : 'transparent',
                        p: 2,
                        alignItems: 'start',
                    }, children: [_jsx(Box, { children: _jsxs(Typography, { variant: 'body1', sx: {
                                    display: 'flex',
                                    alignItems: 'start',
                                    gap: 1,
                                }, children: [_jsx("span", { style: { color: 'primary.main', minWidth: '24px' }, children: `${index + 1}.` }), _jsx("span", { style: { whiteSpace: 'pre-wrap' }, children: pair.original })] }) }), _jsx(Typography, { variant: 'body1', sx: { whiteSpace: 'pre-wrap' }, children: pair.translated })] }, index)))] }));
    };
    return (_jsxs(Stack, { alignItems: 'center', sx: { py: 10, rowGap: 4 }, children: [_jsxs(Box, { textAlign: 'center', display: 'flex', flexDirection: 'row', gap: 2, style: { marginBottom: '20px' }, children: [_jsxs(Button, { variant: 'contained', component: 'label', disabled: state.isLoading, children: ["Select Image", _jsx("input", { type: 'file', accept: 'image/png, image/jpeg, image/jpg', hidden: true, onChange: handleImageSelection })] }), state.image && (_jsx(Button, { variant: 'contained', onClick: handleProcessImage, disabled: state.isLoading, children: "Process Image" }))] }), _jsxs(Box, { display: 'flex', flexDirection: 'row', gap: 4, alignItems: 'center', children: [_jsx(Typography, { variant: 'body1', children: "Result Language:" }), _jsxs(Select, { value: state.selectedLanguage, onChange: (e) => updateState({
                            selectedLanguage: e.target.value,
                        }), disabled: state.isLoading, children: [_jsx(MenuItem, { value: Language.LANGUAGE_EN_US, children: "English" }), _jsx(MenuItem, { value: Language.LANGUAGE_KO_KR, children: "Korean" }), _jsx(MenuItem, { value: Language.LANGUAGE_JA_JP, children: "Japanese" })] })] }), state.image && (_jsx(Box, { sx: { width: '100%', maxWidth: '1200px' }, children: _jsxs(Box, { display: 'flex', flexDirection: 'row', gap: 4, width: '100%', children: [_jsx(Box, { sx: { width: '30%' }, children: _jsx("img", { src: state.result?.translatedImage || state.image.url, alt: state.image.name, style: {
                                    objectFit: 'contain',
                                    width: '100%',
                                    display: 'block',
                                } }) }), _jsx(Box, { sx: {
                                width: '70%',
                                border: '1px solid #ccc',
                                borderRadius: '4px',
                                bgcolor: 'background.paper',
                                p: 2,
                            }, children: state.isLoading ? (_jsxs(Box, { display: 'flex', justifyContent: 'center', alignItems: 'center', p: 4, children: [_jsx(CircularProgress, {}), _jsx(Typography, { sx: { ml: 2 }, children: "Processing image..." })] })) : (state.result &&
                                renderPairedTexts(state.result.sentences?.map((sentence) => sentence.text) ||
                                    [], state.result.sentences?.map((sentence) => sentence.translatedText) || [])) })] }) }))] }));
};
export default ImageToText;
