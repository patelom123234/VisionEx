import { visionex } from './grpcweb/generated/grpc';
export declare enum Language {
    LANGUAGE_EN_US = 1,
    LANGUAGE_KO_KR = 2,
    LANGUAGE_JA_JP = 3
}
export declare enum Model {
    MODEL_GPT4O = 1,
    MODEL_GPT4O_MINI = 2,
    MODEL_GEMINI_FLASH = 3
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
    sentences: ReturnType<typeof visionex.grpc.Sentence.prototype.toObject>[] | null;
    isLoading: boolean;
}
