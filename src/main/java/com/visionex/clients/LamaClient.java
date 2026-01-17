package com.visionex.clients;

import java.awt.image.BufferedImage;

public interface LamaClient {
    BufferedImage createMaskImage(BufferedImage originImage, BufferedImage maskImage) throws Exception;
}

