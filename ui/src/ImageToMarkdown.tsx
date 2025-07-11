import React from 'react';
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
import { Image, Language, Model, TabState } from './type';

const ImageToMarkdown = ({
  state,
  updateState,
}: {
  state: TabState['markdown'];
  updateState: (newState: Partial<TabState['markdown']>) => void;
}) => {
  const translate = async () => {
    updateState({ isLoading: true });
    try {
      if (!state.image) {
        throw new Error('No image selected');
      }
      const markdown = await translateToMarkdown(
        state.image.imageBuffer,
        state.selectedLanguage,
        state.selectedModel,
      );
      updateState({
        markdown: markdown,
        isLoading: false,
      });
    } catch (error) {
      console.error('Error translating image to markdown:', error);
      updateState({ isLoading: false });
    }
  };

  const readFile = (file: File) => {
    return new Promise<Image>((resolve, reject) => {
      const reader: FileReader = new FileReader();

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

    updateState({ isLoading: true });
    try {
      const newImage = await readFile(files[0]);
      updateState({
        image: newImage,
        isLoading: false,
      });
    } catch (error) {
      console.error('Error reading file:', error);
      updateState({ isLoading: false });
    }
  };

  return (
    <Stack
      alignItems='center'
      sx={{
        py: 10,
        rowGap: 4,
      }}
    >
      <Button variant='contained' component='label' disabled={state.isLoading}>
        Select Image
        <input
          type='file'
          accept='image/png, image/jpeg, image/jpg'
          hidden
          onChange={handleImageSelection}
        />
      </Button>

      {state.image && (
        <Box textAlign='center'>
          <Box
            display='flex'
            flexDirection='row'
            flexWrap='wrap'
            justifyContent='center'
            sx={{ mt: 4 }}
          >
            <ImageWithTitle
              src={state.image.url || ''}
              alt={state.image.name}
              title={state.image.name}
            />
          </Box>
          <Box
            display='flex'
            flexDirection='row'
            gap={2}
            justifyContent='center'
          >
            <Typography
              variant='body1'
              sx={{ display: 'flex', alignItems: 'center' }}
            >
              Result Language:
            </Typography>
            <Select
              labelId='language-select-label'
              id='language-select'
              value={state.selectedLanguage}
              onChange={(e) =>
                updateState({
                  selectedLanguage: e.target.value as Language,
                })
              }
            >
              <MenuItem value={Language.LANGUAGE_EN_US}>English</MenuItem>
              <MenuItem value={Language.LANGUAGE_KO_KR}>Korean</MenuItem>
              <MenuItem value={Language.LANGUAGE_JA_JP}>Japanese</MenuItem>
            </Select>
            <Typography
              variant='body1'
              sx={{ display: 'flex', alignItems: 'center' }}
            >
              Model:
            </Typography>
            <Select
              labelId='model-select-label'
              id='model-select'
              value={state.selectedModel}
              onChange={(e) =>
                updateState({
                  selectedModel: e.target.value as Model,
                })
              }
            >
              <MenuItem value={Model.MODEL_GPT4O}>Quality (GPT-4o)</MenuItem>
              <MenuItem value={Model.MODEL_GPT4O_MINI}>
                Economical (GPT-4o Mini)
              </MenuItem>
              <MenuItem value={Model.MODEL_GEMINI_FLASH}>
                Fast (Gemini 1.5 Flash)
              </MenuItem>
            </Select>
            <Button
              onClick={translate}
              variant='contained'
              disabled={state.isLoading}
              sx={{ mx: 2 }}
            >
              CONVERT TO MARKDOWN
            </Button>
          </Box>
        </Box>
      )}

      {state.isLoading && (
        <Box textAlign='center' sx={{ mt: 4 }}>
          <CircularProgress />
          <Typography variant='body1' sx={{ mt: 1 }}>
            Loading image...
          </Typography>
        </Box>
      )}

      {state.markdown && (
        <Box sx={{ mt: 4, display: 'flex', width: '100%', gap: 2 }}>
          <Paper elevation={3} sx={{ flex: 1, p: 2, maxWidth: '50%' }}>
            <Typography variant='h6' gutterBottom>
              Rendered Markdown
            </Typography>
            <ReactMarkdown>{state.markdown}</ReactMarkdown>
          </Paper>
          <Paper elevation={3} sx={{ flex: 1, p: 2, maxWidth: '50%' }}>
            <Typography variant='h6' gutterBottom>
              Raw Markdown
            </Typography>
            <pre style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
              {state.markdown}
            </pre>
          </Paper>
        </Box>
      )}
    </Stack>
  );
};

export default ImageToMarkdown;
