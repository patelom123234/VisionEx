package com.visionex.grpc;

import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.cloud.documentai.v1.Document;
import com.google.cloud.documentai.v1.Document.Page;
import com.google.cloud.documentai.v1.Document.Page.Token;
import com.google.cloud.documentai.v1.Document.Page.Token.StyleInfo;
import com.google.cloud.documentai.v1.Document.TextAnchor.TextSegment;
import com.google.cloud.documentai.v1.OcrConfig;
import com.google.cloud.documentai.v1.ProcessOptions;
import com.google.cloud.documentai.v1.ProcessRequest;
import com.google.cloud.documentai.v1.RawDocument;
import com.google.cloud.vision.v1.TextAnnotation;
import com.google.cloud.vision.v1.TextAnnotation.Page.Block.Paragraph.Word;
import com.google.cloud.vision.v1.Vertex;
import com.google.protobuf.ByteString;
import com.visionex.auth.Auth;
import com.visionex.clients.DocumentAiClient;
import com.visionex.clients.FontProvider;
import com.visionex.clients.FontsByFace;
import com.visionex.clients.GeminiClient;
import com.visionex.clients.LamaClient;
import com.visionex.clients.OpenAiClient;
import com.visionex.clients.OpenAiTranslationClient;
import com.visionex.clients.OpenAiTranslationClient.TargetLanguage;
import com.visionex.clients.StorageClient;
import com.visionex.clients.VisionClient;
import com.visionex.util.ColorDifference;
import io.grpc.Status;
import io.grpc.stub.StreamObserver;
import java.awt.AlphaComposite;
import java.awt.BasicStroke;
import java.awt.Color;
import java.awt.Font;
import java.awt.FontMetrics;
import java.awt.Graphics2D;
import java.awt.RenderingHints;
import java.awt.image.BufferedImage;
import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Base64;
import java.util.Comparator;
import java.util.HashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.Objects;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.Executors;
import java.util.stream.Collectors;
import javax.imageio.ImageIO;

public class VisionExService extends VisionExGrpc.VisionExImplBase {
    private static final String MARKDOWN_PREFIX = "```markdown\n";
    private static final String MARKDOWN_SUFFIX = "\n```";

    private static final double WORD_MERGE_COLOR_DIFF_THRESHOLD = 0.239;
    private static final double LINE_MERGE_COLOR_DIFF_THRESHOLD = 0.08;
    private static final double LINE_MERGE_GRAYSCALE_COLOR_DIFF_THRESHOLD = 0.35;
    private static final double HEIGHT_THRESHOLD = 0.4;
    private static final double HEIGHT_THRESHOLD_TOTAL = 0.125;
    private static final int ADDITIONAL_MASK_PADDING = 8;
    private static final int REGULAR_WEIGHT = 400;
    private static final int SEMIBOLD_WEIGHT = 600;
    private static final int BOLD_WEIGHT = 700;

    private final Auth authClient;
    private final VisionClient visionClient;
    private final OpenAiTranslationClient translationClient;
    private final OpenAiClient openAiClient;
    private final GeminiClient geminiClient;
    private final DocumentAiClient documentAiClient;
    private final DocumentAiSpec documentAiSpec;
    private final Examples examples;
    private final LamaClient lamaClient;
    private final StorageClient storageClient;
    private final String toImageBucket;
    private final String toMarkdownBucket;
    private final FontProvider fontProvider;
    private final java.time.Duration backoffDuration;
    private final ObjectMapper objectMapper = new ObjectMapper();

    public VisionExService(
            Auth authClient,
            VisionClient visionClient,
            OpenAiTranslationClient translationClient,
            OpenAiClient openAiClient,
            GeminiClient geminiClient,
            DocumentAiClient documentAiClient,
            DocumentAiSpec documentAiSpec,
            Examples examples,
            LamaClient lamaClient,
            StorageClient storageClient,
            String toImageBucket,
            String toMarkdownBucket,
            FontProvider fontProvider,
            java.time.Duration backoffDuration) {
        this.authClient = authClient;
        this.visionClient = visionClient;
        this.translationClient = translationClient;
        this.openAiClient = openAiClient;
        this.geminiClient = geminiClient;
        this.documentAiClient = documentAiClient;
        this.documentAiSpec = documentAiSpec;
        this.examples = examples;
        this.lamaClient = lamaClient;
        this.storageClient = storageClient;
        this.toImageBucket = toImageBucket;
        this.toMarkdownBucket = toMarkdownBucket;
        this.fontProvider = fontProvider;
        this.backoffDuration = backoffDuration;
    }

    @Override
    public void signIn(SignInRequest request, StreamObserver<SignInResponse> responseObserver) {
        try {
            String token = request.getGoogleOpenIdToken();
            if (token == null || token.isEmpty()) {
                responseObserver.onError(Status.INVALID_ARGUMENT.withDescription("Google OpenID token is required").asRuntimeException());
                return;
            }
            String verified = authClient.verify(token);
            responseObserver.onNext(SignInResponse.newBuilder().setToken(verified).build());
            responseObserver.onCompleted();
        } catch (Exception ex) {
            responseObserver.onError(Status.PERMISSION_DENIED.withDescription("failed to verify the Google OpenID token").asRuntimeException());
        }
    }

    @Override
    public void translateTextFromImage(TranslateTextFromImageRequest request, StreamObserver<TranslateTextFromImageResponse> responseObserver) {
        try {
            BufferedImage img = ImageIO.read(new ByteArrayInputStream(request.getImage().toByteArray()));
            if (img == null) {
                responseObserver.onError(Status.INVALID_ARGUMENT.withDescription("invalid image").asRuntimeException());
                return;
            }

            TextAnnotation ocrResponse = ocrResult(request.getImage().toByteArray(), img);
            List<WordSegment> wordSegments = textAnnotationToWordSegments(ocrResponse);
            List<ParagraphSegment> paragraphSegments = toParagraphs(wordSegments).stream()
                    .filter(this::containsLetters)
                    .collect(Collectors.toList());

            BufferedImage paragraphImage = new BufferedImage(img.getWidth(), img.getHeight(), BufferedImage.TYPE_INT_ARGB);
            Graphics2D graphics = paragraphImage.createGraphics();
            graphics.drawImage(img, 0, 0, null);
            graphics.dispose();
            drawParagraphBoxesWithNumbers(paragraphImage, paragraphSegments);

            List<String> paragraphTexts = toTexts(paragraphSegments);
            Map<Integer, String> textMap = new HashMap<>();
            for (int i = 0; i < paragraphTexts.size(); i++) {
                textMap.put(i, paragraphTexts.get(i));
            }
            String textJson = objectMapper.writeValueAsString(textMap);

            List<String> translated = translationClient.translate(List.of(textJson), toTargetLanguage(request.getTargetLanguage()));
            if (translated.size() != 1) {
                throw new IllegalStateException("translation failed");
            }
            Map<Integer, String> translatedMap = objectMapper.readValue(translated.get(0), new TypeReference<Map<Integer, String>>() {});
            if (translatedMap.size() != textMap.size()) {
                throw new IllegalStateException("translated text map length mismatch");
            }

            List<Sentence> sentences = new ArrayList<>();
            for (int i = 0; i < translatedMap.size(); i++) {
                sentences.add(Sentence.newBuilder()
                        .setText(textMap.getOrDefault(i, ""))
                        .setTranslatedText(translatedMap.getOrDefault(i, ""))
                        .build());
            }

            String uriImage = "data:image/png;base64," + Base64.getEncoder().encodeToString(toPngBytes(paragraphImage));
            TranslateTextFromImageResponse response = TranslateTextFromImageResponse.newBuilder()
                    .setUriImage(uriImage)
                    .addAllSentences(sentences)
                    .build();
            responseObserver.onNext(response);
            responseObserver.onCompleted();
        } catch (Exception ex) {
            responseObserver.onError(Status.INTERNAL.withDescription(ex.getMessage()).asRuntimeException());
        }
    }

    @Override
    public void translateToMarkdown(TranslateToMarkdownRequest request, StreamObserver<TranslateToMarkdownResponse> responseObserver) {
        try {
            BufferedImage img = ImageIO.read(new ByteArrayInputStream(request.getImage().toByteArray()));
            if (img == null) {
                responseObserver.onError(Status.INVALID_ARGUMENT.withDescription("invalid image").asRuntimeException());
                return;
            }

            ImageSpec spec = new ImageSpec(img.getWidth(), img.getHeight(), request.getImage().toByteArray());
            long timestamp = Instant.now().toEpochMilli();
            storageClient.saveBytes(
                    toMarkdownBucket,
                    String.format(Locale.ROOT, "image-%d-%s-%s-before.png", timestamp, request.getModel(), request.getTargetLanguage()),
                    spec.byteImage
            );

            TextAnnotation ocrText = visionClient.detectDocumentText(spec.byteImage);
            List<WordSegment> wordSegments = textAnnotationToWordSegments(ocrText);
            String alignedText = alignWithSpaces(spec, toParagraphs(wordSegments));

            String markdown = retryWithBackoff(() -> toMarkdown(alignedText, spec.uriImage, request.getModel()));
            String translatedMarkdown = translateMarkdown(markdown, request.getTargetLanguage());

            storageClient.saveBytes(
                    toMarkdownBucket,
                    String.format(Locale.ROOT, "image-%d-%s-%s-after.md", timestamp, request.getModel(), request.getTargetLanguage()),
                    translatedMarkdown.getBytes()
            );

            responseObserver.onNext(TranslateToMarkdownResponse.newBuilder().setMarkdown(translatedMarkdown).build());
            responseObserver.onCompleted();
        } catch (Exception ex) {
            responseObserver.onError(Status.INTERNAL.withDescription(ex.getMessage()).asRuntimeException());
        }
    }

    @Override
    public void translateToImage(TranslateToImageRequest request, StreamObserver<TranslateToImageResponse> responseObserver) {
        try {
            BufferedImage img = ImageIO.read(new ByteArrayInputStream(request.getImage().toByteArray()));
            if (img == null) {
                responseObserver.onError(Status.INVALID_ARGUMENT.withDescription("invalid image").asRuntimeException());
                return;
            }

            ImageSpec spec = new ImageSpec(img.getWidth(), img.getHeight(), request.getImage().toByteArray());
            long timestamp = Instant.now().toEpochMilli();
            storageClient.saveBytes(
                    toImageBucket,
                    String.format(Locale.ROOT, "image-%d-%s-before.png", timestamp, request.getTargetLanguage()),
                    spec.byteImage
            );

            List<ParagraphSegment> paragraphs = detectDocument(spec.byteImage, request.getTargetLanguage());
            var executor = Executors.newFixedThreadPool(2);
            CompletableFuture<BufferedImage> imageWithoutTextsFuture = CompletableFuture.supplyAsync(() -> {
                try {
                    return imageWithoutTexts(spec.byteImage, paragraphs);
                } catch (Exception ex) {
                    throw new RuntimeException(ex);
                }
            }, executor);
            CompletableFuture<List<LineSegment>> translatedFuture = CompletableFuture.supplyAsync(() -> {
                try {
                    return translateParagraphSegments(paragraphs, request.getTargetLanguage());
                } catch (Exception ex) {
                    throw new RuntimeException(ex);
                }
            }, executor);

            BufferedImage imageWithoutTexts = imageWithoutTextsFuture.join();
            List<LineSegment> translatedSegments = translatedFuture.join();
            executor.shutdown();

            BufferedImage translatedImage = drawTexts(imageWithoutTexts, translatedSegments,
                    fontProvider.getFontByLanguage(request.getTargetLanguage()));

            byte[] resultBytes = toPngBytes(translatedImage);
            storageClient.saveBytes(
                    toImageBucket,
                    String.format(Locale.ROOT, "image-%d-%s-after.png", timestamp, request.getTargetLanguage()),
                    resultBytes
            );

            responseObserver.onNext(TranslateToImageResponse.newBuilder()
                    .setUriImage("data:image/png;base64," + Base64.getEncoder().encodeToString(resultBytes))
                    .build());
            responseObserver.onCompleted();
        } catch (Exception ex) {
            responseObserver.onError(Status.INTERNAL.withDescription(ex.getMessage()).asRuntimeException());
        }
    }

    private TextAnnotation ocrResult(byte[] byteImage, BufferedImage img) throws Exception {
        TextAnnotation textAnnotation = visionClient.detectDocumentText(byteImage);
        List<Integer> points = splitPoints(textAnnotation, img.getHeight());
        if (points.size() == 2) {
            return textAnnotation;
        }

        List<TextAnnotation> annotations = new ArrayList<>();
        for (int i = 0; i < points.size() - 1; i++) {
            int start = points.get(i);
            int end = points.get(i + 1);
            BufferedImage subImg = img.getSubimage(0, start, img.getWidth(), end - start);
            byte[] subBytes = toPngBytes(subImg);
            TextAnnotation subText = adjustVerticalPositions(visionClient.detectDocumentText(subBytes), start);
            annotations.add(subText);
        }
        TextAnnotation.Builder merged = TextAnnotation.newBuilder();
        for (TextAnnotation annotation : annotations) {
            merged.addAllPages(annotation.getPagesList());
        }
        return merged.build();
    }

    private List<Integer> splitPoints(TextAnnotation annotations, int imageHeight) {
        final int MAX_HEIGHT = 200;
        List<Page> pages = annotations.getPagesList();
        List<Page.Block> blocks = pages.stream().flatMap(p -> p.getBlocksList().stream()).toList();
        List<Page.Block.Paragraph> paragraphs = blocks.stream().flatMap(b -> b.getParagraphsList().stream()).toList();
        int currentHeight = 0;
        List<Integer> points = new ArrayList<>();
        points.add(0);
        for (Page.Block.Paragraph paragraph : paragraphs) {
            int bottom = paragraph.getBoundingBox().getVerticesList().stream()
                    .map(Vertex::getY)
                    .max(Comparator.naturalOrder())
                    .orElse(0);
            if (bottom - currentHeight > MAX_HEIGHT) {
                points.add(currentHeight);
            }
            currentHeight = bottom;
        }
        if (points.get(points.size() - 1) != imageHeight) {
            points.add(imageHeight);
        }
        return points;
    }

    private TextAnnotation adjustVerticalPositions(TextAnnotation annotation, int offset) {
        TextAnnotation.Builder annotationBuilder = annotation.toBuilder();
        for (int p = 0; p < annotationBuilder.getPagesCount(); p++) {
            Page.Builder pageBuilder = annotationBuilder.getPagesBuilder(p);
            for (int b = 0; b < pageBuilder.getBlocksCount(); b++) {
                Page.Block.Builder blockBuilder = pageBuilder.getBlocksBuilder(b);
                blockBuilder.setBoundingBox(offsetBoundingPoly(blockBuilder.getBoundingBox(), offset));
                for (int pa = 0; pa < blockBuilder.getParagraphsCount(); pa++) {
                    Page.Block.Paragraph.Builder paragraphBuilder = blockBuilder.getParagraphsBuilder(pa);
                    paragraphBuilder.setBoundingBox(offsetBoundingPoly(paragraphBuilder.getBoundingBox(), offset));
                    for (int w = 0; w < paragraphBuilder.getWordsCount(); w++) {
                        Word.Builder wordBuilder = paragraphBuilder.getWordsBuilder(w);
                        wordBuilder.setBoundingBox(offsetBoundingPoly(wordBuilder.getBoundingBox(), offset));
                        for (int s = 0; s < wordBuilder.getSymbolsCount(); s++) {
                            Word.Symbol.Builder symbolBuilder = wordBuilder.getSymbolsBuilder(s);
                            symbolBuilder.setBoundingBox(offsetBoundingPoly(symbolBuilder.getBoundingBox(), offset));
                        }
                    }
                }
            }
        }
        return annotationBuilder.build();
    }

    private com.google.cloud.vision.v1.BoundingPoly offsetBoundingPoly(
            com.google.cloud.vision.v1.BoundingPoly poly,
            int offset) {
        com.google.cloud.vision.v1.BoundingPoly.Builder builder = poly.toBuilder();
        for (int i = 0; i < builder.getVerticesCount(); i++) {
            Vertex vertex = builder.getVertices(i);
            builder.setVertices(i, vertex.toBuilder().setY(vertex.getY() + offset).build());
        }
        return builder.build();
    }

    private List<WordSegment> textAnnotationToWordSegments(TextAnnotation annotation) {
        List<WordSegment> segments = new ArrayList<>();
        for (Page page : annotation.getPagesList()) {
            for (Page.Block block : page.getBlocksList()) {
                for (Page.Block.Paragraph paragraph : block.getParagraphsList()) {
                    for (Word word : paragraph.getWordsList()) {
                        Position position = reduceVertices(word.getBoundingBox().getVerticesList());
                        String text = word.getSymbolsList().stream()
                                .map(Word.Symbol::getText)
                                .collect(Collectors.joining());
                        segments.add(new WordSegment(text, position));
                    }
                }
            }
        }
        return segments;
    }

    private Position reduceVertices(List<Vertex> vertices) {
        int top = Integer.MAX_VALUE;
        int left = Integer.MAX_VALUE;
        int bottom = 0;
        int right = 0;
        for (Vertex v : vertices) {
            top = Math.min(top, v.getY());
            left = Math.min(left, v.getX());
            bottom = Math.max(bottom, v.getY());
            right = Math.max(right, v.getX());
        }
        return new Position(top, left, bottom, right);
    }

    private List<ParagraphSegment> toParagraphs(List<WordSegment> segments) {
        List<LineSegment> lines = new ArrayList<>();
        for (WordSegment word : segments) {
            if (lines.isEmpty()) {
                lines.add(new LineSegment(new ArrayList<>(List.of(word))));
                continue;
            }
            LineSegment lastLine = lines.get(lines.size() - 1);
            WordSegment lastWord = lastLine.words.get(lastLine.words.size() - 1);
            if (isSameLine(lastWord, word)) {
                lastLine.words.add(word);
            } else {
                lines.add(new LineSegment(new ArrayList<>(List.of(word))));
            }
        }

        lines.sort(Comparator.comparingInt(line -> combinedPosition(line.words).top));

        List<ParagraphSegment> paragraphs = new ArrayList<>();
        for (LineSegment line : lines) {
            if (paragraphs.isEmpty()) {
                paragraphs.add(new ParagraphSegment(new ArrayList<>(List.of(line))));
                continue;
            }
            boolean appended = false;
            for (ParagraphSegment paragraph : paragraphs) {
                LineSegment lastLine = paragraph.lines.get(paragraph.lines.size() - 1);
                if (isSameParagraph(lastLine, line)) {
                    paragraph.lines.add(line);
                    appended = true;
                    break;
                }
            }
            if (!appended) {
                paragraphs.add(new ParagraphSegment(new ArrayList<>(List.of(line))));
            }
        }
        return paragraphs;
    }

    private boolean isSameLine(WordSegment previous, WordSegment current) {
        if (previous.position.left > current.position.left) {
            return false;
        }
        int middle = (current.position.top + current.position.bottom) / 2;
        return middle > previous.position.top
                && middle < previous.position.bottom
                && previous.position.right >= current.position.left - (int) (Math.max(charWidth(previous), charWidth(current)) * 1.5);
    }

    private int charWidth(WordSegment word) {
        int letters = 0;
        for (char c : word.text.toCharArray()) {
            if (Character.isLetter(c)) {
                letters++;
            }
        }
        return (word.position.right - word.position.left) / Math.max(letters, 1);
    }

    private boolean isSameParagraph(LineSegment previous, LineSegment current) {
        Position prevPos = combinedPosition(previous.words);
        int prevHeight = prevPos.bottom - prevPos.top;
        Position currPos = combinedPosition(current.words);
        boolean horizontalOverlap = prevPos.right >= currPos.left && prevPos.left <= currPos.right;
        boolean verticalClose = prevPos.bottom + (int) (prevHeight * 0.95) >= currPos.top;
        boolean heightSimilar = Math.abs((currPos.bottom - currPos.top) - prevHeight) <= prevHeight * HEIGHT_THRESHOLD;
        return horizontalOverlap && verticalClose && heightSimilar;
    }

    private Position combinedPosition(List<WordSegment> words) {
        int top = Integer.MAX_VALUE;
        int left = Integer.MAX_VALUE;
        int bottom = 0;
        int right = 0;
        for (WordSegment word : words) {
            Position p = word.position;
            top = Math.min(top, p.top);
            left = Math.min(left, p.left);
            bottom = Math.max(bottom, p.bottom);
            right = Math.max(right, p.right);
        }
        return new Position(top, left, bottom, right);
    }

    private boolean containsLetters(ParagraphSegment paragraph) {
        for (LineSegment line : paragraph.lines) {
            for (WordSegment word : line.words) {
                for (char c : word.text.toCharArray()) {
                    if (Character.isLetter(c)) {
                        return true;
                    }
                }
            }
        }
        return false;
    }

    private void drawParagraphBoxesWithNumbers(BufferedImage img, List<ParagraphSegment> paragraphs) {
        Graphics2D g = img.createGraphics();
        g.setStroke(new BasicStroke(3));
        g.setColor(Color.BLUE);
        for (int i = 0; i < paragraphs.size(); i++) {
            ParagraphSegment paragraph = paragraphs.get(i);
            Position pos = combinedPosition(paragraph.lines.stream()
                    .flatMap(line -> line.words.stream())
                    .collect(Collectors.toList()));
            g.drawRect(pos.left - 3, pos.top - 3, pos.right - pos.left + 6, pos.bottom - pos.top + 6);
            drawNumber(img, pos, i + 1);
        }
        g.dispose();
    }

    private void drawNumber(BufferedImage img, Position pos, int number) {
        Graphics2D g = img.createGraphics();
        g.setColor(Color.RED);
        g.setFont(new Font(Font.SANS_SERIF, Font.PLAIN, 20));
        g.drawString(String.valueOf(number), pos.left, pos.top - 1);
        g.dispose();
    }

    private List<String> toTexts(List<ParagraphSegment> paragraphs) {
        List<String> texts = new ArrayList<>();
        for (ParagraphSegment paragraph : paragraphs) {
            String lines = paragraph.lines.stream()
                    .map(line -> line.words.stream().map(w -> w.text).collect(Collectors.joining(" ")))
                    .collect(Collectors.joining("\n"));
            texts.add(lines);
        }
        return texts;
    }

    private String alignWithSpaces(ImageSpec imageSpec, List<ParagraphSegment> paragraphSegments) throws Exception {
        int minCharHeight = Integer.MAX_VALUE;
        int minCharWidth = Integer.MAX_VALUE;
        for (ParagraphSegment segment : paragraphSegments) {
            List<WordSegment> words = segment.lines.stream().flatMap(line -> line.words.stream()).toList();
            int height = words.stream().mapToInt(w -> w.position.bottom - w.position.top).max().orElse(0);
            int width = words.stream().mapToInt(w -> w.position.right - w.position.left).max().orElse(0);
            if (height == 0 || width == 0) {
                throw new Exception("text segment has invalid dimensions");
            }
            int maxTextLength = segment.lines.stream()
                    .mapToInt(line -> line.words.stream().map(w -> w.text).collect(Collectors.joining()).length())
                    .max().orElse(0);
            if (maxTextLength == 0) {
                throw new Exception("text segment is empty");
            }
            minCharHeight = Math.min(minCharHeight, height / segment.lines.size());
            minCharWidth = Math.min(minCharWidth, width / maxTextLength);
        }

        int gridHeight = (int) Math.ceil((double) imageSpec.height / minCharHeight);
        int gridWidth = (int) Math.ceil((double) imageSpec.width / minCharWidth);
        char[][] textGrid = new char[gridHeight][gridWidth];
        for (int i = 0; i < gridHeight; i++) {
            for (int j = 0; j < gridWidth; j++) {
                textGrid[i][j] = ' ';
            }
        }

        for (ParagraphSegment segment : paragraphSegments) {
            List<WordSegment> words = segment.lines.stream().flatMap(line -> line.words.stream()).toList();
            int left = words.stream().mapToInt(w -> w.position.left).min().orElse(0);
            int top = words.stream().mapToInt(w -> w.position.top).min().orElse(0);
            int startX = (int) ((double) left / minCharWidth);
            int startY = (int) ((double) top / minCharHeight);

            List<String> lines = segment.lines.stream()
                    .map(line -> line.words.stream().map(w -> w.text + " ").collect(Collectors.joining()))
                    .toList();
            if (startY + lines.size() > gridHeight) {
                throw new Exception("text segment exceeds the image height");
            }
            for (int yOffset = 0; yOffset < lines.size(); yOffset++) {
                String line = lines.get(yOffset);
                if (startX + line.length() > gridWidth) {
                    throw new Exception("text segment exceeds the image width");
                }
                for (int xOffset = 0; xOffset < line.length(); xOffset++) {
                    textGrid[startY + yOffset][startX + xOffset] = line.charAt(xOffset);
                }
            }
        }

        StringBuilder result = new StringBuilder();
        int emptyLineCount = 0;
        for (int y = 0; y < gridHeight; y++) {
            String row = new String(textGrid[y]).replaceAll("\\s+$", "");
            if (row.isEmpty()) {
                emptyLineCount++;
                if (emptyLineCount > 2) {
                    continue;
                }
            } else {
                emptyLineCount = 0;
            }
            result.append(row).append("\n");
        }
        return result.toString();
    }

    private String toMarkdown(String text, String base64Image, Model model) throws Exception {
        List<OpenAiClient.ChatMessage> messages = new ArrayList<>();
        messages.add(new OpenAiClient.ChatMessage("system", "The user will provide you with some text information extracted from an image, as well as the image itself.\n"
                + "I need you to take this information and format it into a neat and tidy markdown document.\n"
                + "Please make sure the results are in Markdown format."));
        messages.add(new OpenAiClient.ChatMessage("user", examples.toMarkdownInput));
        messages.add(new OpenAiClient.ChatMessage("assistant", MARKDOWN_PREFIX + examples.toMarkdownOutput + MARKDOWN_SUFFIX));
        messages.add(new OpenAiClient.ChatMessage("user", List.of(
                OpenAiClient.ChatMessagePart.text(text),
                OpenAiClient.ChatMessagePart.imageUrl(base64Image, "high")
        )));

        String content;
        if (model == Model.MODEL_GEMINI_FLASH) {
            content = geminiClient.chatCompletion("gemini-1.5-flash", messages);
        } else {
            String openAiModel = switch (model) {
                case MODEL_GPT4O -> "gpt-4o";
                case MODEL_GPT4O_MINI -> "gpt-4o-mini";
                default -> "gpt-3.5-turbo";
            };
            OpenAiClient.ChatRequest request = new OpenAiClient.ChatRequest(openAiModel, messages);
            content = openAiClient.chatCompletion(request);
        }
        return extractMarkdown(content);
    }

    private String translateMarkdown(String markdown, Language targetLanguage) throws Exception {
        String content = translationClient.chatCompletion(new OpenAiClient.ChatRequest(
                "gpt-4o",
                List.of(
                        new OpenAiClient.ChatMessage("system",
                                "The user will provide you with a markdown document. Please translate the markdown document into " + targetLanguageName(targetLanguage)),
                        new OpenAiClient.ChatMessage("user", markdown)
                )));
        return content;
    }

    private String targetLanguageName(Language language) {
        return switch (language) {
            case LANGUAGE_EN_US -> "American English (United States) (en-US)";
            case LANGUAGE_KO_KR -> "Korean (South Korea) (ko-KR)";
            case LANGUAGE_JA_JP -> "Japanese (Japan) (ja-JP)";
            default -> "American English (United States) (en-US)";
        };
    }

    private String extractMarkdown(String text) throws Exception {
        int start = text.indexOf(MARKDOWN_PREFIX);
        if (start == -1) {
            throw new Exception("no markdown block found");
        }
        int end = text.lastIndexOf(MARKDOWN_SUFFIX);
        if (end == -1) {
            throw new Exception("no closing markdown block found");
        }
        return text.substring(start + MARKDOWN_PREFIX.length(), end);
    }

    private List<ParagraphSegment> detectDocument(byte[] byteImage, Language targetLanguage) throws Exception {
        ProcessRequest request = ProcessRequest.newBuilder()
                .setName(String.format(Locale.ROOT, "projects/%s/locations/%s/processors/%s",
                        documentAiSpec.projectId, documentAiSpec.location, documentAiSpec.processorId))
                .setRawDocument(RawDocument.newBuilder()
                        .setContent(ByteString.copyFrom(byteImage))
                        .setMimeType("image/png")
                        .build())
                .setProcessOptions(ProcessOptions.newBuilder()
                        .setOcrConfig(OcrConfig.newBuilder()
                                .setPremiumFeatures(OcrConfig.PremiumFeatures.newBuilder()
                                        .setComputeStyleInfo(true)
                                        .setEnableSelectionMarkDetection(true)
                                        .build())
                                .build())
                        .build())
                .build();

        Document document = documentAiClient.processDocument(request);
        return groupedSimilarStyle(filterNonTargetLanguage(toDocumentParagraphSegments(document), targetLanguage));
    }

    private BufferedImage drawTexts(BufferedImage image, List<LineSegment> lines, FontsByFace fonts) throws Exception {
        BufferedImage output = new BufferedImage(image.getWidth(), image.getHeight(), BufferedImage.TYPE_INT_ARGB);
        Graphics2D g = output.createGraphics();
        g.drawImage(image, 0, 0, null);
        g.setRenderingHint(RenderingHints.KEY_TEXT_ANTIALIASING, RenderingHints.VALUE_TEXT_ANTIALIAS_ON);

        lines = resizeFont(g, lines, fonts);
        List<WordSegment> words = new ArrayList<>();
        for (LineSegment line : lines) {
            List<Position> positions = line.words.stream().map(w -> w.position).toList();
            words.addAll(repositionText(positions, line.words, g, fonts));
        }

        for (WordSegment word : words) {
            Font font = getFontByWeight(fonts, word.style.fontWeight).deriveFont(word.fontSize.floatValue());
            g.setFont(font);
            g.setColor(word.style.textColor);
            float startX = word.position.left;
            float middleY = (word.position.top + word.position.bottom) / 2f;
            g.drawString(word.text, startX, middleY);
        }
        g.dispose();
        return output;
    }

    private List<LineSegment> resizeFont(Graphics2D g, List<LineSegment> lines, FontsByFace fonts) {
        List<LineSegment> result = new ArrayList<>();
        for (LineSegment line : lines) {
            if (line.words.isEmpty()) {
                result.add(line);
                continue;
            }
            List<Position> combinedPositions = combinedLinePositions(line.words.stream().map(w -> w.position).toList());
            double totalWidth = 0;
            double maxHeight = 0;
            for (Position pos : combinedPositions) {
                totalWidth += pos.right - pos.left;
                maxHeight = Math.max(maxHeight, pos.bottom - pos.top);
            }
            String sentence = line.words.stream().map(w -> w.text).collect(Collectors.joining(" "));
            Font font = getFontByWeight(fonts, line.words.get(0).style.fontWeight);
            double originalSize = line.words.stream()
                    .map(w -> w.fontSize)
                    .filter(Objects::nonNull)
                    .filter(size -> size > 0)
                    .findFirst()
                    .orElse(0.0);
            if (originalSize == 0 && !line.words.isEmpty()) {
                originalSize = line.words.get(0).style.height;
                if (originalSize == 0) {
                    originalSize = 12;
                }
            }
            double lineFontSize = fitFontSize(g, font, sentence, originalSize, new Position(0, 0, (int) maxHeight, (int) totalWidth));
            for (WordSegment word : line.words) {
                if (word.fontSize == null || word.fontSize == 0) {
                    word.fontSize = lineFontSize;
                }
                word.fontSize = Math.min(word.fontSize, lineFontSize);
            }
            result.add(line);
        }
        return result;
    }

    private double fitFontSize(Graphics2D g, Font font, String text, double originalSize, Position box) {
        double boxWidth = box.right - box.left;
        double boxHeight = box.bottom - box.top;
        g.setFont(font.deriveFont((float) originalSize));
        FontMetrics metrics = g.getFontMetrics();
        double width = metrics.stringWidth(text);
        double height = metrics.getHeight();
        if (width <= boxWidth && height <= boxHeight) {
            return originalSize;
        }
        double low = 1.0;
        double high = originalSize;
        while (low <= high) {
            double mid = (low + high) / 2;
            g.setFont(font.deriveFont((float) mid));
            metrics = g.getFontMetrics();
            width = metrics.stringWidth(text);
            height = metrics.getHeight();
            if (width <= boxWidth && height <= boxHeight) {
                low = mid + 1;
            } else {
                high = mid - 1;
            }
        }
        return Math.max(1, high);
    }

    private List<WordSegment> repositionText(List<Position> currentPositions, List<WordSegment> words, Graphics2D g, FontsByFace fonts) {
        List<Position> combinedPositions = combinedLinePositions(currentPositions);
        List<WordSegment> queue = words.stream()
                .map(w -> new WordSegment(w.text + " ", w.position, w.style, w.fontSize))
                .collect(Collectors.toCollection(ArrayList::new));
        List<WordSegment> result = new ArrayList<>();
        for (Position pos : combinedPositions) {
            if (queue.isEmpty()) {
                break;
            }
            int remainWidth = pos.right - pos.left;
            Position current = new Position(pos.top, pos.left, pos.bottom, pos.right);
            while (!queue.isEmpty()) {
                WordSegment word = queue.remove(0);
                Font font = getFontByWeight(fonts, word.style.fontWeight).deriveFont(word.fontSize.floatValue());
                g.setFont(font);
                int width = g.getFontMetrics().stringWidth(word.text);
                if (width <= remainWidth) {
                    result.add(new WordSegment(word.text, new Position(current.top, current.left, current.bottom, current.left + width), word.style, word.fontSize));
                    current.left += width;
                    remainWidth -= width;
                } else {
                    int availableCount = 0;
                    for (int i = 1; i <= word.text.length(); i++) {
                        int w = g.getFontMetrics().stringWidth(word.text.substring(0, i));
                        if (w > remainWidth) {
                            availableCount = i - 1;
                            break;
                        }
                    }
                    String front = word.text.substring(0, availableCount);
                    String tail = word.text.substring(availableCount);
                    result.add(new WordSegment(front, new Position(current.top, current.left, current.bottom, current.right), word.style, word.fontSize));
                    queue.add(0, new WordSegment(tail, word.position, word.style, word.fontSize));
                    break;
                }
            }
        }
        return result;
    }

    private Font getFontByWeight(FontsByFace fonts, int weight) {
        if (weight >= BOLD_WEIGHT) {
            return fonts.bold;
        } else if (weight >= SEMIBOLD_WEIGHT) {
            return fonts.semiBold;
        }
        return fonts.regular;
    }

    private List<Position> combinedLinePositions(List<Position> positions) {
        List<Position> combined = new ArrayList<>();
        for (Position current : positions) {
            if (combined.isEmpty()) {
                combined.add(current);
                continue;
            }
            Position last = combined.get(combined.size() - 1);
            int middle = (last.top + last.bottom) / 2;
            if (current.top <= middle && current.bottom >= middle) {
                combined.set(combined.size() - 1, combinedPosition(List.of(
                        new WordSegment("", last), new WordSegment("", current)
                )));
            } else {
                combined.add(current);
            }
        }
        return combined;
    }

    private List<LineSegment> translateParagraphSegments(List<ParagraphSegment> paragraphSegments, Language targetLanguage) throws Exception {
        List<LineSegment> lines = retryWithBackoff(() -> groupedLines(paragraphSegments));
        List<List<LineSegment>> splitLines = new ArrayList<>();
        while (lines.size() > 1) {
            splitLines.add(lines.subList(0, 2));
            lines = lines.subList(2, lines.size());
        }
        if (!lines.isEmpty()) {
            splitLines.add(lines);
        }

        List<LineSegment> result = new ArrayList<>();
        for (List<LineSegment> split : splitLines) {
            int[] id = {0};
            List<List<SegmentWithId>> textSegments = split.stream()
                    .map(line -> line.words.stream()
                            .map(word -> new SegmentWithId(++id[0], word.text, word.style, word.position, word.fontSize))
                            .toList())
                    .toList();

            List<List<SegmentWithId>> translatedSegments = retryWithBackoff(() -> translate(textSegments, targetLanguage));
            validateTranslatedSegments(textSegments, translatedSegments);

            List<SegmentWithId> originalSegments = textSegments.stream()
                    .flatMap(List::stream)
                    .collect(Collectors.toList());

            List<LineSegment> mapped = new ArrayList<>();
            int[] idx = {0};
            for (List<SegmentWithId> translated : translatedSegments) {
                List<WordSegment> words = new ArrayList<>();
                for (SegmentWithId segment : translated) {
                    SegmentWithId matched = originalSegments.stream()
                            .filter(original -> original.id == segment.id)
                            .findFirst()
                            .orElse(originalSegments.get(idx[0]));
                    idx[0]++;
                    words.add(new WordSegment(segment.text, matched.position, matched.style, matched.fontSize));
                }
                mapped.add(new LineSegment(words));
            }
            result.addAll(mapped);
        }
        return result;
    }

    private List<List<SegmentWithId>> translate(List<List<SegmentWithId>> segments, Language targetLanguage) throws Exception {
        String payload = objectMapper.writeValueAsString(segments.stream()
                .map(list -> list.stream().map(SegmentWithId::toTransport).toList())
                .toList());

        String response = translationClient.chatCompletion(new OpenAiClient.ChatRequest(
                "gpt-4o",
                List.of(
                        new OpenAiClient.ChatMessage("system", "The user provides a word and an ID for each sentence.\n"
                                + "You will translate those words and assign an ID based on the translated word. Please translate into " + targetLanguageName(targetLanguage) + ".\n"
                                + "Each array is one statement. Please translate it naturally into one sentence.\n"
                                + "If you determine that the object should disappear, do not destroy the object, but return only the text as an empty string with original ID.\n"
                                + "The order of ID could be changed, but the ID should never disappear. Example: { \"id\": 1, \"text\": \"\" }\n"
                                + "Please do not miss special characters, etc.\n"
                                + "Please only send json responses. Examples include:\n"
                                + "[\n"
                                + "[ { \"id\": 1234, \"text\": \"Translated word\" } ],\n"
                                + "[ { \"id\": 1122, \"text\": \"Translated word2\" } ],\n"
                                + "]\n"),
                        new OpenAiClient.ChatMessage("user", "[ [ { \"id\": 1, \"text\": \"밥\" }, { \"id\": 2, \"text\": \"먹으러\" }, { \"id\": 3, \"text\": \"가자\" } ] ]"),
                        new OpenAiClient.ChatMessage("assistant", "[ [ { \"id\": 3, \"text\": \"Let's\" }, { \"id\": 2, \"text\": \"go\" }, { \"id\": 1, \"text\": \"eat\" } ] ]"),
                        new OpenAiClient.ChatMessage("user", payload)
                )));

        List<List<SegmentWithId>> translatedSegments = objectMapper.readValue(response, new TypeReference<List<List<SegmentWithId>>>() {});
        return translatedSegments;
    }

    private void validateTranslatedSegments(List<List<SegmentWithId>> original, List<List<SegmentWithId>> translated) throws Exception {
        if (original.size() != translated.size()) {
            throw new Exception("invalid response length");
        }
        for (int i = 0; i < translated.size(); i++) {
            if (original.get(i).size() != translated.get(i).size()) {
                throw new Exception("invalid response length");
            }
            List<Integer> originalIds = original.get(i).stream().map(s -> s.id).sorted().toList();
            List<Integer> translatedIds = translated.get(i).stream().map(s -> s.id).sorted().toList();
            for (int j = 0; j < originalIds.size(); j++) {
                if (!Objects.equals(originalIds.get(j), translatedIds.get(j))) {
                    throw new Exception("invalid id");
                }
            }
        }
    }

    private List<LineSegment> groupedLines(List<ParagraphSegment> paragraphSegments) throws Exception {
        List<ParagraphSegment> oneLine = paragraphSegments.stream()
                .filter(p -> p.lines.size() == 1)
                .toList();
        List<ParagraphSegment> multiLine = paragraphSegments.stream()
                .filter(p -> p.lines.size() > 1)
                .toList();
        if (multiLine.isEmpty()) {
            return paragraphSegments.stream().flatMap(p -> p.lines.stream()).toList();
        }

        PromptValue prompt = toPromptValue(multiLine);
        String response = translationClient.chatCompletion(new OpenAiClient.ChatRequest(
                "gpt-4o",
                List.of(
                        new OpenAiClient.ChatMessage("system", "The user provides a list of texts inside each paragraph.\n"
                                + "You should return the list of texts within each paragraph by grouping them by sentence.\n"
                                + "Please return them by grouping them by sentence. If it doesn't look natural when concatenated, don't group them and return them individually. Please watch it very closely.\n"
                                + "In other words, if it is natural without being tied together, it must exist individually.\n"
                                + "I emphasize this again. If it exists naturally as a single sentence, do not combine it with another sentence. You should only merge if you are sure.\n"
                                + "Group the user-provided text into sentences. There should be no missing text. Provide the IDs of the matching user-provided texts in JSON format.\n"
                                + "For example:\n"
                                + "[[{ \"id\": 0, \"text\": \"에버랜드앱에서 \\\"가상줄서기\\\" 신청 후\" },{ \"id\": 1, \"text\": \"예약된 시간에 이용하는 서비스입니다.\" }],\n"
                                + "[{ \"id\": 2, \"text\": \"※ 에버랜드에서는 입장객이 많을 경우 안전을 위해 조기 오픈하여 입장할 수 있습니다.\" },{ \"id\": 3, \"text\": \"(조기 오픈여부 및 시간은 당일 상황에 따라 결정됨)\" },{ \"id\": 4, \"text\": \"조기 오픈시 입장 후 일부 시설에 대해 스마트줄서기 신청이 가능하며 조기 마감될 수 있습니다.\" },{ \"id\": 5, \"text\": \"각 시설별 운영시간은 에버랜드 모바일APP에서 확인하실 수 있습니다.\" },{ \"id\": 6, \"text\": \"※ 스마트 줄서기 시설 마감시 14시 이후 현장 줄서기로 이용 가능합니다.(일부시설 제외)\" }],\n"
                                + "[{ \"id\": 7, \"text\": \"※기상상황 및 운영상황에 따라 어트랙션 운영 및 공연이 변경 또는 취소될 수 있으니\" },{ \"id\": 8, \"text\": \"자세한 내용은 에버랜드 홈페이지 또는 APP에서 확인 바랍니다.\" }],\n"
                                + "[{ \"id\": 9, \"text\": \"에버랜드\" },{ \"id\": 10, \"text\": \"즐길거리\" }]]\n"
                                + "So, you have to provide only the ID of the text provided by the matched user in JSON format.\n"
                                + "Example:\n"
                                + "```json\n"
                                + "[\n"
                                + "[ [0, 1] ],\n"
                                + "[ [2], [3], [4], [5], [6] ],\n"
                                + "[ [7, 8] ],\n"
                                + "[ [9], [10] ]\n"
                                + "]\n"
                                + "```\n"),
                        new OpenAiClient.ChatMessage("user", examples.groupedLinesInput),
                        new OpenAiClient.ChatMessage("assistant", examples.groupedLinesOutput),
                        new OpenAiClient.ChatMessage("user", prompt.json)
                )));

        String jsonResponse = extractJson(response);
        List<List<List<Integer>>> groupedIds = objectMapper.readValue(jsonResponse, new TypeReference<List<List<List<Integer>>>>() {});
        validateIds(prompt.originalLines, groupedIds);

        List<LineSegment> result = new ArrayList<>();
        result.addAll(oneLine.stream().flatMap(p -> p.lines.stream()).toList());
        for (List<List<Integer>> lineIds : groupedIds) {
            double fontSize = 0;
            for (List<Integer> ids : lineIds) {
                LineSegment combined = new LineSegment(new ArrayList<>());
                for (int id : ids) {
                    LineSegment originalLine = prompt.originalLines.get(id);
                    for (WordSegment word : originalLine.words) {
                        combined.words.add(new WordSegment(word.text, word.position, word.style, fontSize));
                    }
                }
                result.add(combined);
            }
        }
        return result;
    }

    private void validateIds(List<LineSegment> lines, List<List<List<Integer>>> groupedIds) throws Exception {
        List<Integer> ids = groupedIds.stream()
                .flatMap(List::stream)
                .flatMap(List::stream)
                .toList();
        if (ids.size() != lines.size()) {
            throw new Exception("invalid response length");
        }
        for (int i = 0; i < lines.size(); i++) {
            if (!ids.contains(i)) {
                throw new Exception("invalid id");
            }
        }
    }

    private PromptValue toPromptValue(List<ParagraphSegment> paragraphs) throws Exception {
        int id = 0;
        List<LineSegment> originalLines = new ArrayList<>();
        List<List<Map<String, Object>>> paragraphsWithIds = new ArrayList<>();
        for (ParagraphSegment paragraph : paragraphs) {
            List<Map<String, Object>> wordWithIds = new ArrayList<>();
            for (LineSegment line : paragraph.lines) {
                String text = line.words.stream().map(w -> w.text).collect(Collectors.joining());
                wordWithIds.add(Map.of("id", id, "text", text));
                originalLines.add(line);
                id++;
            }
            paragraphsWithIds.add(wordWithIds);
        }
        String json = objectMapper.writeValueAsString(paragraphsWithIds);
        return new PromptValue(originalLines, json);
    }

    private String extractJson(String text) throws Exception {
        int startIndex = text.indexOf("```json");
        if (startIndex == -1) {
            throw new Exception("no JSON block in the text");
        }
        int endIndex = text.lastIndexOf("```");
        if (endIndex == -1) {
            throw new Exception("no closing JSON block in the text");
        }
        return text.substring(startIndex + "```json".length(), endIndex);
    }

    private List<ParagraphSegment> toDocumentParagraphSegments(Document document) {
        List<WordSegment> words = new ArrayList<>();
        for (Page page : document.getPagesList()) {
            for (Token token : page.getTokensList()) {
                Position position = reduceDocVertices(token.getLayout().getBoundingPoly().getVerticesList());
                String text = token.getLayout().getTextAnchor().getTextSegmentsList().stream()
                        .map(segment -> substringBySegment(document.getText(), segment))
                        .collect(Collectors.joining());

                StyleInfo styleInfo = token.getStyleInfo();
                int fontWeight = (int) styleInfo.getFontWeight();
                boolean isBold = styleInfo.getBold();
                if (fontWeight == 0) {
                    fontWeight = isBold ? BOLD_WEIGHT : REGULAR_WEIGHT;
                }
                double fontSize = styleInfo.getPixelFontSize();

                Color textColor = new Color(
                        clampColor(styleInfo.getTextColor().getRed()),
                        clampColor(styleInfo.getTextColor().getGreen()),
                        clampColor(styleInfo.getTextColor().getBlue())
                );
                words.add(new WordSegment(
                        text.replace("\n", ""),
                        position,
                        new Style(textColor, position.bottom - position.top, 1, fontWeight),
                        fontSize
                ));
            }
        }
        return toParagraphs(words);
    }

    private Position reduceDocVertices(List<com.google.cloud.documentai.v1.Vertex> vertices) {
        int top = Integer.MAX_VALUE;
        int left = Integer.MAX_VALUE;
        int bottom = 0;
        int right = 0;
        for (com.google.cloud.documentai.v1.Vertex v : vertices) {
            top = Math.min(top, v.getY());
            left = Math.min(left, v.getX());
            bottom = Math.max(bottom, v.getY());
            right = Math.max(right, v.getX());
        }
        return new Position(top, left, bottom, right);
    }

    private String substringBySegment(String text, TextSegment segment) {
        int start = (int) segment.getStartIndex();
        int end = (int) segment.getEndIndex();
        if (start < 0 || end > text.length() || start >= end) {
            return "";
        }
        return text.substring(start, end);
    }

    private List<ParagraphSegment> groupedSimilarStyle(List<ParagraphSegment> segments) {
        List<ParagraphSegment> result = new ArrayList<>();
        for (ParagraphSegment paragraph : segments) {
            if (paragraph.lines.isEmpty()) {
                result.add(paragraph);
                continue;
            }
            List<LineSegment> lines = groupBlackLines(paragraph.lines.stream()
                    .map(this::groupSimilarTextByLine)
                    .toList());
            List<LineSegment> combined = new ArrayList<>();
            for (LineSegment line : lines) {
                if (combined.isEmpty()) {
                    combined.add(line);
                    continue;
                }
                LineSegment lastLine = combined.get(combined.size() - 1);
                if (shouldCombineLine(lastLine, line)) {
                    for (WordSegment word : line.words) {
                        word.style = lastLine.words.get(lastLine.words.size() - 1).style;
                    }
                }
                combined.add(line);
            }
            result.add(new ParagraphSegment(combined));
        }
        return result;
    }

    private List<LineSegment> groupBlackLines(List<LineSegment> lines) {
        List<LineSegment> result = new ArrayList<>();
        for (LineSegment line : lines) {
            List<WordSegment> combined = new ArrayList<>();
            for (WordSegment word : line.words) {
                if (combined.isEmpty()) {
                    combined.add(word);
                    continue;
                }
                WordSegment last = combined.get(combined.size() - 1);
                if (shouldTreatAsBlack(last) && shouldTreatAsBlack(word)) {
                    combined.set(combined.size() - 1, combineWordSegments(last, word));
                } else {
                    combined.add(word);
                }
            }
            result.add(new LineSegment(combined));
        }
        return result;
    }

    private boolean shouldTreatAsBlack(WordSegment word) {
        Color black = new Color(0, 0, 0);
        Color gray = new Color(128, 128, 128);
        Color silver = new Color(192, 192, 192);
        return colorDistance(word.style.textColor, black) < 0.2
                || colorDistance(word.style.textColor, gray) < 0.2
                || colorDistance(word.style.textColor, silver) < 0.2;
    }

    private LineSegment groupSimilarTextByLine(LineSegment line) {
        List<WordSegment> combined = new ArrayList<>();
        for (WordSegment word : line.words) {
            if (combined.isEmpty()) {
                combined.add(word);
                continue;
            }
            WordSegment last = combined.get(combined.size() - 1);
            if (shouldCombineWord(last, word)) {
                combined.set(combined.size() - 1, combineWordSegments(last, word));
            } else {
                combined.add(word);
            }
        }
        return new LineSegment(combined);
    }

    private boolean shouldCombineWord(WordSegment previous, WordSegment current) {
        if (isOnlySymbol(previous.text.trim()) || isOnlySymbol(current.text.trim())) {
            return true;
        }
        return isSimilar(previous.style, current.style, HEIGHT_THRESHOLD, WORD_MERGE_COLOR_DIFF_THRESHOLD);
    }

    private boolean shouldCombineLine(LineSegment previous, LineSegment current) {
        if (previous.words.isEmpty() || current.words.isEmpty()) {
            return false;
        }
        if (current.words.stream().anyMatch(word -> current.words.get(current.words.size() - 1).style != word.style)) {
            return false;
        }
        if (previous.words.stream().anyMatch(word -> previous.words.get(previous.words.size() - 1).style != word.style)) {
            return false;
        }
        WordSegment prevLast = previous.words.get(previous.words.size() - 1);
        WordSegment currLast = current.words.get(current.words.size() - 1);
        if (Math.abs(prevLast.style.height - currLast.style.height) > prevLast.style.height * HEIGHT_THRESHOLD_TOTAL) {
            return false;
        }
        double colorThreshold = isGrayscaleColor(prevLast.style.textColor) && isGrayscaleColor(currLast.style.textColor)
                ? LINE_MERGE_GRAYSCALE_COLOR_DIFF_THRESHOLD
                : LINE_MERGE_COLOR_DIFF_THRESHOLD;
        return colorDistance(prevLast.style.textColor, currLast.style.textColor) <= colorThreshold;
    }

    private boolean isGrayscaleColor(Color color) {
        double maxDiff = Math.max(Math.max(Math.abs(color.getRed() - color.getGreen()), Math.abs(color.getGreen() - color.getBlue())),
                Math.abs(color.getBlue() - color.getRed()));
        return maxDiff < 25.5;
    }

    private WordSegment combineWordSegments(WordSegment previous, WordSegment current) {
        Style maintained = new Style(
                blendColor(previous.style.textColor, current.style.textColor, (double) previous.style.weight / (previous.style.weight + current.style.weight)),
                (previous.style.height + current.style.height) / 2,
                previous.style.weight + current.style.weight,
                previous.style.fontWeight
        );
        if (isOnlySymbol(previous.text.trim())) {
            maintained = current.style;
        } else if (isOnlySymbol(current.text.trim())) {
            maintained = previous.style;
        }
        double fontSize = previous.fontSize;
        return new WordSegment(previous.text + current.text, combinedPosition(List.of(previous, current)), maintained, fontSize);
    }

    private boolean isSimilar(Style previous, Style current, double heightThreshold, double colorThreshold) {
        if (Math.abs(previous.height - current.height) > previous.height * heightThreshold) {
            return false;
        }
        double actualThreshold = colorThreshold;
        if (colorThreshold == LINE_MERGE_COLOR_DIFF_THRESHOLD
                && isGrayscaleColor(previous.textColor)
                && isGrayscaleColor(current.textColor)) {
            actualThreshold = LINE_MERGE_GRAYSCALE_COLOR_DIFF_THRESHOLD;
        }
        return colorDistance(previous.textColor, current.textColor) <= actualThreshold;
    }

    private boolean isOnlySymbol(String text) {
        for (char c : text.toCharArray()) {
            if (isLanguage(c)) {
                return false;
            }
        }
        return true;
    }

    private boolean isLanguage(char c) {
        Character.UnicodeBlock block = Character.UnicodeBlock.of(c);
        return block == Character.UnicodeBlock.HANGUL_SYLLABLES
                || block == Character.UnicodeBlock.HIRAGANA
                || block == Character.UnicodeBlock.KATAKANA
                || block == Character.UnicodeBlock.BASIC_LATIN
                || Character.isLetter(c);
    }

    private BufferedImage imageWithoutTexts(byte[] byteImage, List<ParagraphSegment> paragraphs) throws Exception {
        BufferedImage originImage = ImageIO.read(new ByteArrayInputStream(byteImage));
        if (originImage == null) {
            throw new Exception("failed to decode image");
        }
        BufferedImage mask = new BufferedImage(originImage.getWidth(), originImage.getHeight(), BufferedImage.TYPE_INT_ARGB);
        Graphics2D g = mask.createGraphics();
        g.setComposite(AlphaComposite.Src);
        g.setColor(new Color(255, 255, 255, 255));
        for (LineSegment line : paragraphs.stream().flatMap(p -> p.lines.stream()).toList()) {
            Position position = combinedPosition(line.words);
            int left = Math.max(position.left - ADDITIONAL_MASK_PADDING, 0);
            int top = Math.max(position.top - ADDITIONAL_MASK_PADDING, 0);
            int right = Math.min(position.right + ADDITIONAL_MASK_PADDING, mask.getWidth());
            int bottom = Math.min(position.bottom + ADDITIONAL_MASK_PADDING, mask.getHeight());
            g.fillRect(left, top, right - left, bottom - top);
        }
        g.dispose();
        return lamaClient.createMaskImage(originImage, mask);
    }

    private List<ParagraphSegment> filterNonTargetLanguage(List<ParagraphSegment> paragraphSegments, Language targetLanguage) {
        List<ParagraphSegment> filtered = new ArrayList<>();
        for (ParagraphSegment paragraph : paragraphSegments) {
            List<LineSegment> lines = new ArrayList<>();
            for (LineSegment line : paragraph.lines) {
                String text = line.words.stream().map(w -> w.text).collect(Collectors.joining());
                for (char c : text.toCharArray()) {
                    Language detected = detectedLanguage(c);
                    if (detected != Language.LANGUAGE_UNSPECIFIED && detected != targetLanguage) {
                        lines.add(line);
                        break;
                    }
                }
            }
            filtered.add(new ParagraphSegment(lines));
        }
        return filtered;
    }

    private Language detectedLanguage(char c) {
        Character.UnicodeBlock block = Character.UnicodeBlock.of(c);
        if (block == Character.UnicodeBlock.HANGUL_SYLLABLES) {
            return Language.LANGUAGE_KO_KR;
        }
        if (block == Character.UnicodeBlock.HIRAGANA || block == Character.UnicodeBlock.KATAKANA) {
            return Language.LANGUAGE_JA_JP;
        }
        if (block == Character.UnicodeBlock.BASIC_LATIN) {
            return Language.LANGUAGE_EN_US;
        }
        if (Character.isLetter(c)) {
            return Language.LANGUAGE_UNSPECIFIED;
        }
        return Language.LANGUAGE_UNSPECIFIED;
    }

    private TargetLanguage toTargetLanguage(Language language) {
        return switch (language) {
            case LANGUAGE_KO_KR -> TargetLanguage.KO_KR;
            case LANGUAGE_JA_JP -> TargetLanguage.JA_JP;
            default -> TargetLanguage.EN_US;
        };
    }

    private byte[] toPngBytes(BufferedImage image) throws Exception {
        ByteArrayOutputStream out = new ByteArrayOutputStream();
        ImageIO.write(image, "png", out);
        return out.toByteArray();
    }

    private int clampColor(float value) {
        return Math.min(255, Math.max(0, Math.round(value * 255)));
    }

    private Color blendColor(Color a, Color b, double ratio) {
        int r = (int) Math.round(a.getRed() * ratio + b.getRed() * (1 - ratio));
        int g = (int) Math.round(a.getGreen() * ratio + b.getGreen() * (1 - ratio));
        int bl = (int) Math.round(a.getBlue() * ratio + b.getBlue() * (1 - ratio));
        return new Color(r, g, bl);
    }

    private double colorDistance(Color c1, Color c2) {
        return ColorDifference.ciede2000(c1, c2);
    }

    private <T> T retryWithBackoff(ThrowingSupplier<T> supplier) throws Exception {
        int retries = 4;
        Exception last = null;
        for (int i = 0; i < retries; i++) {
            try {
                return supplier.get();
            } catch (Exception ex) {
                last = ex;
                Thread.sleep(backoffDuration.toMillis());
            }
        }
        throw last != null ? last : new Exception("retry failed");
    }

    public static class DocumentAiSpec {
        public final String projectId;
        public final String location;
        public final String processorId;

        public DocumentAiSpec(String projectId, String location, String processorId) {
            this.projectId = projectId;
            this.location = location;
            this.processorId = processorId;
        }
    }

    public static class Examples {
        public final String toMarkdownInput;
        public final String toMarkdownOutput;
        public final String groupedLinesInput;
        public final String groupedLinesOutput;

        public Examples(String toMarkdownInput, String toMarkdownOutput, String groupedLinesInput, String groupedLinesOutput) {
            this.toMarkdownInput = toMarkdownInput;
            this.toMarkdownOutput = toMarkdownOutput;
            this.groupedLinesInput = groupedLinesInput;
            this.groupedLinesOutput = groupedLinesOutput;
        }
    }

    private static class ImageSpec {
        private final int width;
        private final int height;
        private final String uriImage;
        private final byte[] byteImage;

        private ImageSpec(int width, int height, byte[] byteImage) {
            this.width = width;
            this.height = height;
            this.byteImage = byteImage;
            this.uriImage = "data:image/png;base64," + Base64.getEncoder().encodeToString(byteImage);
        }
    }

    private static class ParagraphSegment {
        private final List<LineSegment> lines;

        private ParagraphSegment(List<LineSegment> lines) {
            this.lines = lines;
        }
    }

    private static class LineSegment {
        private final List<WordSegment> words;

        private LineSegment(List<WordSegment> words) {
            this.words = words;
        }
    }

    private static class WordSegment {
        private final String text;
        private final Position position;
        private Style style;
        private Double fontSize;

        private WordSegment(String text, Position position) {
            this(text, position, null, null);
        }

        private WordSegment(String text, Position position, Style style, Double fontSize) {
            this.text = text;
            this.position = position;
            this.style = style;
            this.fontSize = fontSize;
        }
    }

    private static class Position {
        private int top;
        private int left;
        private int bottom;
        private int right;

        private Position(int top, int left, int bottom, int right) {
            this.top = top;
            this.left = left;
            this.bottom = bottom;
            this.right = right;
        }
    }

    private static class Style {
        private final Color textColor;
        private final int height;
        private final int weight;
        private final int fontWeight;

        private Style(Color textColor, int height, int weight, int fontWeight) {
            this.textColor = textColor;
            this.height = height;
            this.weight = weight;
            this.fontWeight = fontWeight;
        }
    }

    private static class SegmentWithId {
        public int id;
        public String text;
        public transient Style style;
        public transient Position position;
        public transient Double fontSize;

        private SegmentWithId() {}

        private SegmentWithId(int id, String text, Style style, Position position, Double fontSize) {
            this.id = id;
            this.text = text;
            this.style = style;
            this.position = position;
            this.fontSize = fontSize;
        }

        private Map<String, Object> toTransport() {
            return Map.of("id", id, "text", text);
        }
    }

    private static class PromptValue {
        private final List<LineSegment> originalLines;
        private final String json;

        private PromptValue(List<LineSegment> originalLines, String json) {
            this.originalLines = originalLines;
            this.json = json;
        }
    }

    @FunctionalInterface
    private interface ThrowingSupplier<T> {
        T get() throws Exception;
    }
}

