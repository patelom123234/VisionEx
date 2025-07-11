import { visionex } from './grpcweb/generated/grpc';

export enum Language {
	LANGUAGE_EN_US = visionex.grpc.Language.LANGUAGE_EN_US,
	LANGUAGE_KO_KR = visionex.grpc.Language.LANGUAGE_KO_KR,
	LANGUAGE_JA_JP = visionex.grpc.Language.LANGUAGE_JA_JP,
}

export enum Model {
	MODEL_GPT4O = visionex.grpc.Model.MODEL_GPT4O,
	MODEL_GPT4O_MINI = visionex.grpc.Model.MODEL_GPT4O_MINI,
	MODEL_GEMINI_FLASH = visionex.grpc.Model.MODEL_GEMINI_FLASH,
}

export interface Image {
  id?: string;
  name: string;
  imageBuffer: Uint8Array;
  url?: string;
}

export interface TabState {
  markdown: {
    image: Image | null;
    markdown: string | null;
    selectedLanguage: Language;
    selectedModel: Model;
    isLoading: boolean;
  };
  image: {
    image: Image | null;
    translatedImage: string | null;
    selectedLanguage: Language;
    isLoading: boolean;
  };
  text: {
    selectedLanguage: Language;
    image: Image | null;
    result: ProcessedResult | null;
    isLoading: boolean;
  };
}

export interface ProcessedResult {
  id: string | undefined;
  originalImage: Image;
  translatedImage: string | undefined;
  sentences:
    	| ReturnType<typeof visionex.grpc.Sentence.prototype.toObject>[]
    | null;
  isLoading: boolean;
}
