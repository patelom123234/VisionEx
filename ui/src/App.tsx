import { useState } from 'react';
import Stack from '@mui/material/Stack';
import Tab from '@mui/material/Tab';
import Tabs from '@mui/material/Tabs';
import ImageToImage from './ImageToImage';
import ImageToMarkdown from './ImageToMarkdown';
import ImageToText from './ImageToText';
import SignIn from './Signin';
import { VISIONEX_TOKEN_KEY } from './grpcweb/client';
import { Language, Model, TabState } from './type';

const App = () => {
  const [currentTab, setCurrentTab] = useState(0);
  const [isAuthenticated, setIsAuthenticated] = useState(
    		!!localStorage.getItem(VISIONEX_TOKEN_KEY),
  );
  const [tabState, setTabState] = useState<TabState>({
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

  const updateTabState = <K extends keyof TabState>(
    tab: K,
    newState: Partial<TabState[K]>,
  ) => {
    setTabState((prev: TabState) => ({
      ...prev,
      [tab]: { ...prev[tab], ...newState },
    }));
  };

  return !isAuthenticated ? (
    <SignIn setAuthenticated={setIsAuthenticated} />
  ) : (
    <Stack>
      <Tabs
        value={currentTab}
        onChange={(_, newValue) => setCurrentTab(newValue)}
      >
        <Tab label='Markdown' />
        <Tab label='Image' />
        <Tab label='Text' />
      </Tabs>
      {currentTab === 0 && (
        <ImageToMarkdown
          state={tabState.markdown}
          updateState={(newState) => updateTabState('markdown', newState)}
        />
      )}
      {currentTab === 1 && (
        <ImageToImage
          state={tabState.image}
          updateState={(newState) => updateTabState('image', newState)}
        />
      )}
      {currentTab === 2 && (
        <ImageToText
          state={tabState.text}
          updateState={(newState) => updateTabState('text', newState)}
        />
      )}
    </Stack>
  );
};

export default App;
