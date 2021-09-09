from setuptools import setup

setup(
    name='Badger',
    version='0.1.0',
    author='Róisín Grannell',
    author_email='r.grannell2@gmail.com',

    # a racoon?
    scripts=['bin/badger'],
    url='http://pypi.python.org/pypi/PackageName/',
    license='LICENSE.txt',
    description='Badger is a tool that helps you filter large folders of photos',
    long_description=open('README.md').read(),
    install_requires=[
        "numpy",
        "opencv-python",
        "docopt",
        "sklearn",
        "Pillow"
    ],
)
