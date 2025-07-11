import React from 'react';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import CircularProgress from '@mui/material/CircularProgress';
import MenuItem from '@mui/material/MenuItem';
import Select from '@mui/material/Select';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import { translateTextFromImage } from './grpcweb/client';
import { Image, Language, ProcessedResult, TabState } from './type';

const ImageToText = ({
  state,
  updateState,
}: {
  state: TabState['text'];
  updateState: (newState: Partial<TabState['text']>) => void;
}) => {
  const readFile = (file: File): Promise<Image> => {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => {
        if (reader.result instanceof ArrayBuffer) {
          const imageBuffer = new Uint8Array(reader.result);
          const url = URL.createObjectURL(
            new Blob([imageBuffer], { type: 'image/png' }),
          );
          resolve({
            name: file.name,
            imageBuffer,
            url,
            id: Math.random().toString(36).substring(2, 15),
          });
        } else {
          reject(new Error('Failed to read file.'));
        }
      };
      reader.onerror = () => reject(new Error('File reading failed.'));
      reader.readAsArrayBuffer(file);
    });
  };

  const handleImageSelection = async (
    event: React.ChangeEvent<HTMLInputElement>,
  ) => {
    const files = event.target.files;
    if (!files || files.length === 0) return;

    try {
      const newImage = await readFile(files[0]);
      updateState({
        image: newImage,
        result: null,
      });
    } catch (error) {
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
    } catch (error) {
      console.error('Error processing image:', error);
      updateState({ isLoading: false });
    }
  };

  const processImage = async (image: Image): Promise<ProcessedResult> => {
    try {
      const imageData = await translateTextFromImage(
        image.imageBuffer,
        state.selectedLanguage,
      );

      return {
        id: image.id,
        originalImage: image,
        translatedImage: imageData.uriImage,
        sentences: imageData.sentences,
        isLoading: false,
      };
    } catch (error) {
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

  const renderPairedTexts = (
    originalTexts: (string | undefined)[],
    translatedTexts: (string | undefined)[],
  ) => {
    const maxLength = Math.max(originalTexts.length, translatedTexts.length);
    const pairs = Array.from({ length: maxLength }, (_, i) => ({
      original: originalTexts[i] || '',
      translated: translatedTexts[i] || '',
    }));

    return (
      <Box sx={{ width: '100%' }}>
        <Box
          sx={{
            display: 'grid',
            gridTemplateColumns: '1fr 1fr',
            gap: 2,
            bgcolor: 'background.paper',
            borderRadius: 1,
            p: 2,
            borderBottom: '2px solid #eee',
          }}
        >
          <Typography variant='h6' sx={{ fontWeight: 'bold' }}>
            Original Text
          </Typography>
          <Typography variant='h6' sx={{ fontWeight: 'bold' }}>
            Translated Text
          </Typography>
        </Box>
        {pairs.map((pair, index) => (
          <Box
            key={index}
            sx={{
              display: 'grid',
              gridTemplateColumns: '1fr 1fr',
              gap: 2,
              bgcolor: index % 2 === 0 ? 'rgba(0, 0, 0, 0.02)' : 'transparent',
              p: 2,
              alignItems: 'start',
            }}
          >
            <Box>
              <Typography
                variant='body1'
                sx={{
                  display: 'flex',
                  alignItems: 'start',
                  gap: 1,
                }}
              >
                <span
                  style={{ color: 'primary.main', minWidth: '24px' }}
                >{`${index + 1}.`}</span>
                <span style={{ whiteSpace: 'pre-wrap' }}>{pair.original}</span>
              </Typography>
            </Box>
            <Typography variant='body1' sx={{ whiteSpace: 'pre-wrap' }}>
              {pair.translated}
            </Typography>
          </Box>
        ))}
      </Box>
    );
  };

  return (
    <Stack alignItems='center' sx={{ py: 10, rowGap: 4 }}>
      <Box
        textAlign='center'
        display='flex'
        flexDirection='row'
        gap={2}
        style={{ marginBottom: '20px' }}
      >
        <Button
          variant='contained'
          component='label'
          disabled={state.isLoading}
        >
          Select Image
          <input
            type='file'
            accept='image/png, image/jpeg, image/jpg'
            hidden
            onChange={handleImageSelection}
          />
        </Button>
        {state.image && (
          <Button
            variant='contained'
            onClick={handleProcessImage}
            disabled={state.isLoading}
          >
            Process Image
          </Button>
        )}
      </Box>

      <Box display='flex' flexDirection='row' gap={4} alignItems='center'>
        <Typography variant='body1'>Result Language:</Typography>
        <Select
          value={state.selectedLanguage}
          onChange={(e) =>
            updateState({
              selectedLanguage: e.target.value as Language,
            })
          }
          disabled={state.isLoading}
        >
          <MenuItem value={Language.LANGUAGE_EN_US}>English</MenuItem>
          <MenuItem value={Language.LANGUAGE_KO_KR}>Korean</MenuItem>
          <MenuItem value={Language.LANGUAGE_JA_JP}>Japanese</MenuItem>
        </Select>
      </Box>

      {state.image && (
        <Box sx={{ width: '100%', maxWidth: '1200px' }}>
          <Box display='flex' flexDirection='row' gap={4} width='100%'>
            <Box sx={{ width: '30%' }}>
              <img
                src={state.result?.translatedImage || state.image.url}
                alt={state.image.name}
                style={{
                  objectFit: 'contain',
                  width: '100%',
                  display: 'block',
                }}
              />
            </Box>
            <Box
              sx={{
                width: '70%',
                border: '1px solid #ccc',
                borderRadius: '4px',
                bgcolor: 'background.paper',
                p: 2,
              }}
            >
              {state.isLoading ? (
                <Box
                  display='flex'
                  justifyContent='center'
                  alignItems='center'
                  p={4}
                >
                  <CircularProgress />
                  <Typography sx={{ ml: 2 }}>Processing image...</Typography>
                </Box>
              ) : (
                state.result &&
                renderPairedTexts(
                  state.result.sentences?.map((sentence) => sentence.text) ||
                    [],
                  state.result.sentences?.map(
                    (sentence) => sentence.translatedText,
                  ) || [],
                )
              )}
            </Box>
          </Box>
        </Box>
      )}
    </Stack>
  );
};

export default ImageToText;
