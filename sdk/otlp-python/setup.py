# demo_dog Python SDK — installable via `pip install -e .`.
from setuptools import setup, find_packages
setup(
    name="demo-dog",
    version="0.1.0",
    description="Stdlib-only OTLP-style client for the demo-dog collector.",
    packages=find_packages(),
    python_requires=">=3.8",
    classifiers=[
        "Programming Language :: Python :: 3",
        "License :: OSI Approved :: Apache Software License",
        "Operating System :: OS Independent",
    ],
)
